// Package platform defines the abstraction for Git hosting platforms
// and the registry that maps platform IDs to their implementations.
package platform

import (
	"context"
	"gh-mirror/pkg/models"
)

// Platform is the interface that every supported Git hosting platform must implement.
type Platform interface {
	ID() models.PlatformID
	Name() string
	Configure(token string, apiURL string, webURL string, owner string) error

	GetAuthenticatedUser(ctx context.Context) (string, error)
	ListRepositories(ctx context.Context) ([]models.Repository, error)
	GetRepository(ctx context.Context, owner, repo string) (*models.Repository, error)
	CreateRepository(ctx context.Context, name string, private bool, description string) (*models.Repository, error)
	UpdateRepository(ctx context.Context, owner, repo string, private bool, description string) error
	RepositoryExists(ctx context.Context, owner, repo string) (bool, error)

	// CloneURL builds a clone/push URL for the given repository.
	// Authentication is handled separately via http.BasicAuth.
	CloneURL(repo models.Repository) string

	// CleanPullRefs removes pull-request references (refs/pull/*) from a cloned
	// repository before pushing, as they are platform-specific and should not be
	// mirrored.
	CleanPullRefs(repoPath string) error
}
