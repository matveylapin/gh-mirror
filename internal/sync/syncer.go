// Package sync implements the core repository mirroring logic between platforms.
package sync

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gh-mirror/internal/git"
	"gh-mirror/pkg/models"
	"gh-mirror/pkg/platform"
)

// Credential holds the authentication and endpoint details for a single platform.
type Credential struct {
	Token  string
	APIURL string
	URL    string
	Owner  string
}

// Credentials maps platform IDs to their credentials.
type Credentials map[models.PlatformID]Credential

// Syncer orchestrates repository mirroring from a source platform to one or more destinations.
type Syncer struct {
	source       platform.Platform
	destinations []platform.Platform
	destUsers    map[models.PlatformID]string
	creds        Credentials
	logger       *slog.Logger
	tempDir      string
	sourceUser   string
}

// NewSyncer creates a new Syncer and initializes its temporary working directory.
func NewSyncer(source platform.Platform, destinations []platform.Platform, creds Credentials, logger *slog.Logger) (*Syncer, error) {
	tempDir, err := os.MkdirTemp("", "gh-mirror-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	return &Syncer{
		source:       source,
		destinations: destinations,
		creds:        creds,
		logger:       logger,
		tempDir:      tempDir,
		destUsers:    make(map[models.PlatformID]string),
	}, nil
}

// Close removes the temporary working directory. Should be called when the Syncer is no longer needed.
func (s *Syncer) Close() error {
	return os.RemoveAll(s.tempDir)
}

// Init authenticates with all platforms and resolves usernames.
// Must be called before SyncAll, SyncOne, ListRepositories, or ListDiff.
func (s *Syncer) Init(ctx context.Context) error {
	var err error
	s.sourceUser, err = s.source.GetAuthenticatedUser(ctx)
	if err != nil {
		return fmt.Errorf("get source username: %w", err)
	}

	for _, dest := range s.destinations {
		destUser, err := dest.GetAuthenticatedUser(ctx)
		if err != nil {
			return fmt.Errorf("get %s username: %w", dest.ID(), err)
		}
		s.destUsers[dest.ID()] = destUser
	}

	s.logger.Info("initialized",
		"source", s.source.ID(),
		"source_user", s.sourceUser,
		"destinations", s.destinationIDs(),
	)

	return nil
}

// effectiveOwner returns the configured owner for a platform, falling back to the
// authenticated username. Used when calling platform API methods that accept an
// owner parameter (GetRepository, RepositoryExists, UpdateRepository).
func (s *Syncer) effectiveOwner(pID models.PlatformID, authUser string) string {
	if c, ok := s.creds[pID]; ok && c.Owner != "" {
		return c.Owner
	}
	return authUser
}

func (s *Syncer) destinationIDs() []models.PlatformID {
	ids := make([]models.PlatformID, len(s.destinations))
	for i, d := range s.destinations {
		ids[i] = d.ID()
	}
	return ids
}

// SyncAll mirrors all repositories from the source platform to every destination.
func (s *Syncer) SyncAll(ctx context.Context) ([]models.SyncResult, error) {
	s.logger.Info("starting full sync")

	sourceRepos, err := s.source.ListRepositories(ctx)
	if err != nil {
		return nil, fmt.Errorf("list source repositories: %w", err)
	}

	destRepoMaps := make(map[models.PlatformID]map[string]models.Repository)
	for _, dest := range s.destinations {
		destRepos, err := dest.ListRepositories(ctx)
		if err != nil {
			return nil, fmt.Errorf("list %s repositories: %w", dest.ID(), err)
		}
		repoMap := make(map[string]models.Repository)
		for _, r := range destRepos {
			repoMap[r.Name] = r
		}
		destRepoMaps[dest.ID()] = repoMap
	}

	var results []models.SyncResult

	for _, srcRepo := range sourceRepos {
		for _, dest := range s.destinations {
			destRepoMap := destRepoMaps[dest.ID()]
			var destRepo models.Repository
			if existing, exists := destRepoMap[srcRepo.Name]; exists {
				destRepo = existing
			}
			result := s.syncRepository(ctx, srcRepo, dest, destRepo)
			results = append(results, result)
		}
	}

	s.logger.Info("sync completed",
		"total", len(results),
		"created", countActions(results, models.ActionCreate),
		"updated", countActions(results, models.ActionUpdate),
		"skipped", countActions(results, models.ActionSkip),
	)

	return results, nil
}

// SyncOne mirrors a single named repository from the source platform to all destinations.
func (s *Syncer) SyncOne(ctx context.Context, repoName string) ([]models.SyncResult, error) {
	sourceOwner := s.effectiveOwner(s.source.ID(), s.sourceUser)
	srcRepo, err := s.source.GetRepository(ctx, sourceOwner, repoName)
	if err != nil {
		return nil, fmt.Errorf("get source repository: %w", err)
	}

	var results []models.SyncResult

	for _, dest := range s.destinations {
		destUser := s.destUsers[dest.ID()]
		destOwner := s.effectiveOwner(dest.ID(), destUser)

		exists, err := dest.RepositoryExists(ctx, destOwner, repoName)
		if err != nil {
			results = append(results, models.SyncResult{
				RepoName:    repoName,
				Destination: dest.ID(),
				Action:      models.ActionSkip,
				Error:       err,
				Message:     "failed to check repository",
			})
			continue
		}

		var destRepo models.Repository
		if exists {
			repo, err := dest.GetRepository(ctx, destOwner, repoName)
			if err != nil {
				results = append(results, models.SyncResult{
					RepoName:    repoName,
					Destination: dest.ID(),
					Action:      models.ActionSkip,
					Error:       err,
					Message:     "failed to get repository",
				})
				continue
			}
			destRepo = *repo
		}

		result := s.syncRepository(ctx, *srcRepo, dest, destRepo)
		results = append(results, result)
	}

	return results, nil
}

func (s *Syncer) syncRepository(ctx context.Context, srcRepo models.Repository, dest platform.Platform, destRepo models.Repository) models.SyncResult {
	s.logger.Info("syncing repository",
		"name", srcRepo.Name,
		"source", s.source.ID(),
		"destination", dest.ID(),
		"private", srcRepo.Private,
	)

	action := models.ActionUpdate
	if destRepo.Name == "" {
		action = models.ActionCreate
	}

	if action == models.ActionUpdate {
		s.logger.Info("checking refs for changes", "name", srcRepo.Name)

		sourceRefs, err := s.getRemoteRefs(srcRepo, s.creds[s.source.ID()].Token, s.source)
		if err != nil {
			return models.SyncResult{
				RepoName:    srcRepo.Name,
				Destination: dest.ID(),
				Action:      action,
				Error:       err,
				Message:     "failed to get source refs",
			}
		}

		destRefs, err := s.getRemoteRefs(srcRepo, s.creds[dest.ID()].Token, dest)
		if err != nil {
			// Destination repo exists but may be empty (no branches/tags yet).
			// Treat as empty refs and proceed with push.
			s.logger.Warn("failed to get destination refs, treating as empty", "name", srcRepo.Name, "error", err)
			destRefs = make(map[string]string)
		}

		inSync, reason := compareRefs(sourceRefs, destRefs)
		if inSync {
			s.logger.Info("repository already in sync", "name", srcRepo.Name)
			return models.SyncResult{
				RepoName:    srcRepo.Name,
				Destination: dest.ID(),
				Action:      models.ActionSkip,
				Message:     "already in sync",
			}
		}

		s.logger.Info("refs differ, will sync", "name", srcRepo.Name, "reason", reason)
	}

	if action == models.ActionCreate {
		_, err := dest.CreateRepository(ctx, srcRepo.Name, srcRepo.Private, srcRepo.Description)
		if err != nil {
			return models.SyncResult{
				RepoName:    srcRepo.Name,
				Destination: dest.ID(),
				Action:      action,
				Error:       err,
				Message:     "failed to create",
			}
		}
		s.logger.Info("created repository", "name", srcRepo.Name, "destination", dest.ID())
	} else {
		destUser := s.destUsers[dest.ID()]
		destOwner := s.effectiveOwner(dest.ID(), destUser)
		if destRepo.Private != srcRepo.Private || destRepo.Description != srcRepo.Description {
			if err := dest.UpdateRepository(ctx, destOwner, srcRepo.Name, srcRepo.Private, srcRepo.Description); err != nil {
				return models.SyncResult{
					RepoName:    srcRepo.Name,
					Destination: dest.ID(),
					Action:      action,
					Error:       err,
					Message:     "failed to update",
				}
			}
			s.logger.Info("updated repository",
				"name", srcRepo.Name,
				"destination", dest.ID(),
				"private", srcRepo.Private,
				"description", srcRepo.Description,
			)
		}
	}

	if err := s.pushMirror(srcRepo, dest); err != nil {
		return models.SyncResult{
			RepoName:    srcRepo.Name,
			Destination: dest.ID(),
			Action:      action,
			Error:       err,
			Message:     "failed to push mirror",
		}
	}

	return models.SyncResult{
		RepoName:    srcRepo.Name,
		Destination: dest.ID(),
		Action:      action,
		Message:     "synced successfully",
	}
}

func (s *Syncer) pushMirror(repo models.Repository, dest platform.Platform) error {
	safeName := filepath.Base(repo.Name)
	repoPath := git.GetRepoPath(s.tempDir, safeName)

	cloneURL := s.source.CloneURL(repo)

	repoHandle, err := git.Clone(cloneURL, repoPath, s.creds[s.source.ID()].Token)
	if err != nil {
		return fmt.Errorf("git clone: %w", err)
	}

	if err := s.source.CleanPullRefs(repoPath); err != nil {
		return fmt.Errorf("delete pull refs: %w", err)
	}

	pushURL := dest.CloneURL(repo)

	if err := git.Push(repoHandle, "origin", pushURL, s.creds[dest.ID()].Token, true); err != nil {
		return fmt.Errorf("git push mirror: %w", err)
	}

	return nil
}

// ListRepositories returns all repositories from the source platform.
func (s *Syncer) ListRepositories(ctx context.Context) ([]models.Repository, error) {
	return s.source.ListRepositories(ctx)
}

// ListDiff compares repositories between the source platform and all destinations,
// reporting missing, extra, or visibility-mismatched repositories.
func (s *Syncer) ListDiff(ctx context.Context) ([]models.DiffItem, error) {
	sourceRepos, err := s.source.ListRepositories(ctx)
	if err != nil {
		return nil, fmt.Errorf("list source repositories: %w", err)
	}

	sourceMap := make(map[string]models.Repository)
	for _, r := range sourceRepos {
		sourceMap[r.Name] = r
	}

	var diff []models.DiffItem

	if len(s.destinations) == 0 {
		return diff, nil
	}

	destRepoMaps := make(map[models.PlatformID]map[string]models.Repository)
	for _, dest := range s.destinations {
		destRepos, err := dest.ListRepositories(ctx)
		if err != nil {
			return nil, fmt.Errorf("list %s repositories: %w", dest.ID(), err)
		}
		repoMap := make(map[string]models.Repository)
		for _, r := range destRepos {
			repoMap[r.Name] = r
		}
		destRepoMaps[dest.ID()] = repoMap
	}

	for _, dest := range s.destinations {
		destMap := destRepoMaps[dest.ID()]

		for name, srcRepo := range sourceMap {
			destRepo, exists := destMap[name]
			if !exists {
				diff = append(diff, models.DiffItem{
					Name:                 name,
					Source:               &srcRepo,
					Destination:          nil,
					DestinationPlatform:  dest.ID(),
					Description:           fmt.Sprintf("missing on %s", dest.ID()),
				})
			} else if srcRepo.Private != destRepo.Private {
				diff = append(diff, models.DiffItem{
					Name:                 name,
					Source:               &srcRepo,
					Destination:          &destRepo,
					DestinationPlatform:  dest.ID(),
					Description:           fmt.Sprintf("visibility mismatch: %s=%v, %s=%v", s.source.ID(), srcRepo.Private, dest.ID(), destRepo.Private),
				})
			}
		}

		for name, destRepo := range destMap {
			if _, exists := sourceMap[name]; !exists {
				diff = append(diff, models.DiffItem{
					Name:                 name,
					Source:               nil,
					Destination:          &destRepo,
					DestinationPlatform:  dest.ID(),
					Description:          fmt.Sprintf("only on %s", dest.ID()),
				})
			}
		}
	}

	return diff, nil
}

func (s *Syncer) getRemoteRefs(repo models.Repository, token string, p platform.Platform) (map[string]string, error) {
	cloneURL := p.CloneURL(repo)

	refs, err := git.ListRemote(cloneURL, token)
	if err != nil {
		return nil, fmt.Errorf("git ls-remote %s: %w", p.ID(), err)
	}

	return refs, nil
}

func compareRefs(sourceRefs, destRefs map[string]string) (bool, string) {
	for ref, sourceSHA := range sourceRefs {
		destSHA, exists := destRefs[ref]
		if !exists {
			return false, fmt.Sprintf("ref %s missing on destination", ref)
		}
		if sourceSHA != destSHA {
			return false, fmt.Sprintf("SHA mismatch for %s", ref)
		}
	}

	return true, ""
}

func countActions(results []models.SyncResult, action models.SyncAction) int {
	count := 0
	for _, r := range results {
		if r.Action == action {
			count++
		}
	}
	return count
}
