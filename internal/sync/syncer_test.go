package sync

import (
	"context"
	"log/slog"
	"testing"

	"github.com/nalgeon/be"

	"gh-mirror/pkg/models"
	"gh-mirror/pkg/platform"
)

type MockPlatform struct {
	idFunc               func() models.PlatformID
	nameFunc             func() string
	configureFunc        func(token, apiURL, webURL, owner string) error
	getAuthUserFunc      func(ctx context.Context) (string, error)
	listReposFunc        func(ctx context.Context) ([]models.Repository, error)
	getRepoFunc          func(ctx context.Context, owner, repo string) (*models.Repository, error)
	createRepoFunc       func(ctx context.Context, name string, private bool, description string) (*models.Repository, error)
	updateRepoFunc       func(ctx context.Context, owner, repo string, private bool, description string) error
	repoExistsFunc       func(ctx context.Context, owner, repo string) (bool, error)
	cloneURLFunc         func(repo models.Repository) string
	cleanPullRefsFunc    func(repoPath string) error
}

func (m *MockPlatform) ID() models.PlatformID                                     { return m.idFunc() }
func (m *MockPlatform) Name() string                                              { return m.nameFunc() }
func (m *MockPlatform) Configure(token, apiURL, webURL, owner string) error { return m.configureFunc(token, apiURL, webURL, owner) }
func (m *MockPlatform) GetAuthenticatedUser(ctx context.Context) (string, error)  { return m.getAuthUserFunc(ctx) }
func (m *MockPlatform) ListRepositories(ctx context.Context) ([]models.Repository, error) {
	return m.listReposFunc(ctx)
}
func (m *MockPlatform) GetRepository(ctx context.Context, owner, repo string) (*models.Repository, error) {
	return m.getRepoFunc(ctx, owner, repo)
}
func (m *MockPlatform) CreateRepository(ctx context.Context, name string, private bool, description string) (*models.Repository, error) {
	return m.createRepoFunc(ctx, name, private, description)
}
func (m *MockPlatform) UpdateRepository(ctx context.Context, owner, repo string, private bool, description string) error {
	return m.updateRepoFunc(ctx, owner, repo, private, description)
}
func (m *MockPlatform) RepositoryExists(ctx context.Context, owner, repo string) (bool, error) {
	return m.repoExistsFunc(ctx, owner, repo)
}
func (m *MockPlatform) CloneURL(repo models.Repository) string     { return m.cloneURLFunc(repo) }
func (m *MockPlatform) CleanPullRefs(repoPath string) error                       { return m.cleanPullRefsFunc(repoPath) }

func newSourceMock() *MockPlatform {
	return &MockPlatform{
		idFunc:            func() models.PlatformID { return "github" },
		nameFunc:          func() string { return "GitHub" },
		configureFunc:     func(_, _, _, _ string) error { return nil },
		getAuthUserFunc:   func(_ context.Context) (string, error) { return "srcuser", nil },
		listReposFunc:     func(_ context.Context) ([]models.Repository, error) { return nil, nil },
		getRepoFunc:       func(_ context.Context, _, _ string) (*models.Repository, error) { return nil, nil },
		createRepoFunc:    func(_ context.Context, _ string, _ bool, _ string) (*models.Repository, error) { return nil, nil },
		updateRepoFunc:    func(_ context.Context, _, _ string, _ bool, _ string) error { return nil },
		repoExistsFunc:    func(_ context.Context, _, _ string) (bool, error) { return false, nil },
		cloneURLFunc:      func(_ models.Repository) string { return "https://github.com/user/repo.git" },
		cleanPullRefsFunc: func(_ string) error { return nil },
	}
}

func newDestMock(id models.PlatformID) *MockPlatform {
	return &MockPlatform{
		idFunc:            func() models.PlatformID { return id },
		nameFunc:          func() string { return string(id) },
		configureFunc:     func(_, _, _, _ string) error { return nil },
		getAuthUserFunc:   func(_ context.Context) (string, error) { return "destuser", nil },
		listReposFunc:     func(_ context.Context) ([]models.Repository, error) { return nil, nil },
		getRepoFunc:       func(_ context.Context, _, _ string) (*models.Repository, error) { return nil, nil },
		createRepoFunc:    func(_ context.Context, _ string, _ bool, _ string) (*models.Repository, error) { return nil, nil },
		updateRepoFunc:    func(_ context.Context, _, _ string, _ bool, _ string) error { return nil },
		repoExistsFunc:    func(_ context.Context, _, _ string) (bool, error) { return false, nil },
		cloneURLFunc:      func(_ models.Repository) string { return "https://example.com/user/repo.git" },
		cleanPullRefsFunc: func(_ string) error { return nil },
	}
}

func testCredentials() Credentials {
	return Credentials{
		models.PlatformID("github"): {Token: "ghp_test", URL: "https://github.com"},
		models.PlatformID("gitlab"): {Token: "glpat_test", URL: "https://gitlab.com"},
	}
}

func TestNewSyncer(t *testing.T) {
	src := newSourceMock()
	dests := []platform.Platform{newDestMock("gitlab")}
	cfg := testCredentials()
	logger := slog.Default()

	s, err := NewSyncer(src, dests, cfg, logger)
	be.True(t, err == nil)
	be.True(t, s != nil)
	be.Equal(t, s.tempDir != "", true)

	err = s.Close()
	be.True(t, err == nil)
}

func TestNewSyncerCreatesTempDir(t *testing.T) {
	src := newSourceMock()
	dests := []platform.Platform{newDestMock("gitlab")}
	cfg := testCredentials()
	logger := slog.Default()

	s, err := NewSyncer(src, dests, cfg, logger)
	be.True(t, err == nil)
	tempDir := s.tempDir
	be.True(t, tempDir != "")

	s.Close()
}

func TestCloseRemovesTempDir(t *testing.T) {
	src := newSourceMock()
	dests := []platform.Platform{newDestMock("gitlab")}
	cfg := testCredentials()
	logger := slog.Default()

	s, err := NewSyncer(src, dests, cfg, logger)
	be.True(t, err == nil)
	tempDir := s.tempDir

	err = s.Close()
	be.True(t, err == nil)

	be.Equal(t, tempDir, s.tempDir)
}

func TestInit(t *testing.T) {
	src := newSourceMock()
	dest := newDestMock("gitlab")
	dests := []platform.Platform{dest}
	cfg := testCredentials()
	logger := slog.Default()

	s, err := NewSyncer(src, dests, cfg, logger)
	be.True(t, err == nil)
	defer s.Close()

	err = s.Init(context.Background())
	be.True(t, err == nil)
	be.Equal(t, s.sourceUser, "srcuser")
	be.Equal(t, s.destUsers[models.PlatformID("gitlab")], "destuser")
}

func TestInitSourceAuthFails(t *testing.T) {
	src := newSourceMock()
	src.getAuthUserFunc = func(_ context.Context) (string, error) {
		return "", platform.ErrNotAuthenticated
	}
	dests := []platform.Platform{newDestMock("gitlab")}
	cfg := testCredentials()
	logger := slog.Default()

	s, err := NewSyncer(src, dests, cfg, logger)
	be.True(t, err == nil)
	defer s.Close()

	err = s.Init(context.Background())
	be.True(t, err != nil)
}

func TestInitDestAuthFails(t *testing.T) {
	src := newSourceMock()
	dest := newDestMock("gitlab")
	dest.getAuthUserFunc = func(_ context.Context) (string, error) {
		return "", platform.ErrNotAuthenticated
	}
	dests := []platform.Platform{dest}
	cfg := testCredentials()
	logger := slog.Default()

	s, err := NewSyncer(src, dests, cfg, logger)
	be.True(t, err == nil)
	defer s.Close()

	err = s.Init(context.Background())
	be.True(t, err != nil)
}

func TestDestinationIDs(t *testing.T) {
	src := newSourceMock()
	gitlabDest := newDestMock("gitlab")
	codebergDest := newDestMock("codeberg")
	dests := []platform.Platform{gitlabDest, codebergDest}

	s, err := NewSyncer(src, dests, testCredentials(), slog.Default())
	be.True(t, err == nil)
	defer s.Close()

	ids := s.destinationIDs()
	be.Equal(t, len(ids), 2)
	be.Equal(t, ids[0], models.PlatformID("gitlab"))
	be.Equal(t, ids[1], models.PlatformID("codeberg"))
}

func TestListDiffMissingOnDest(t *testing.T) {
	src := newSourceMock()
	src.listReposFunc = func(_ context.Context) ([]models.Repository, error) {
		return []models.Repository{
			{Name: "repo1", Private: false},
			{Name: "repo2", Private: true},
		}, nil
	}

	dest := newDestMock("gitlab")
	dest.listReposFunc = func(_ context.Context) ([]models.Repository, error) {
		return []models.Repository{}, nil
	}

	s, err := NewSyncer(src, []platform.Platform{dest}, testCredentials(), slog.Default())
	be.True(t, err == nil)
	defer s.Close()

	diff, err := s.ListDiff(context.Background())
	be.True(t, err == nil)
	be.Equal(t, len(diff), 2)

	names := make(map[string]bool)
	for _, item := range diff {
		names[item.Name] = true
		be.True(t, item.Source != nil)
		be.True(t, item.Destination == nil)
	}
	be.True(t, names["repo1"])
	be.True(t, names["repo2"])
}

func TestListDiffOnlyOnDest(t *testing.T) {
	src := newSourceMock()
	src.listReposFunc = func(_ context.Context) ([]models.Repository, error) {
		return []models.Repository{}, nil
	}

	dest := newDestMock("gitlab")
	dest.listReposFunc = func(_ context.Context) ([]models.Repository, error) {
		return []models.Repository{
			{Name: "orphan-repo", Private: true},
		}, nil
	}

	s, err := NewSyncer(src, []platform.Platform{dest}, testCredentials(), slog.Default())
	be.True(t, err == nil)
	defer s.Close()

	diff, err := s.ListDiff(context.Background())
	be.True(t, err == nil)
	be.Equal(t, len(diff), 1)
	be.Equal(t, diff[0].Name, "orphan-repo")
	be.True(t, diff[0].Source == nil)
	be.True(t, diff[0].Destination != nil)
}

func TestListDiffVisibilityMismatch(t *testing.T) {
	src := newSourceMock()
	src.listReposFunc = func(_ context.Context) ([]models.Repository, error) {
		return []models.Repository{
			{Name: "repo1", Private: true},
		}, nil
	}

	dest := newDestMock("gitlab")
	dest.listReposFunc = func(_ context.Context) ([]models.Repository, error) {
		return []models.Repository{
			{Name: "repo1", Private: false},
		}, nil
	}

	s, err := NewSyncer(src, []platform.Platform{dest}, testCredentials(), slog.Default())
	be.True(t, err == nil)
	defer s.Close()

	diff, err := s.ListDiff(context.Background())
	be.True(t, err == nil)
	be.Equal(t, len(diff), 1)
	be.Equal(t, diff[0].Name, "repo1")
	be.True(t, diff[0].Source != nil)
	be.True(t, diff[0].Destination != nil)
}

func TestListDiffInSync(t *testing.T) {
	src := newSourceMock()
	src.listReposFunc = func(_ context.Context) ([]models.Repository, error) {
		return []models.Repository{
			{Name: "repo1", Private: false},
		}, nil
	}

	dest := newDestMock("gitlab")
	dest.listReposFunc = func(_ context.Context) ([]models.Repository, error) {
		return []models.Repository{
			{Name: "repo1", Private: false},
		}, nil
	}

	s, err := NewSyncer(src, []platform.Platform{dest}, testCredentials(), slog.Default())
	be.True(t, err == nil)
	defer s.Close()

	diff, err := s.ListDiff(context.Background())
	be.True(t, err == nil)
	be.Equal(t, len(diff), 0)
}

func TestListDiffNoDestinations(t *testing.T) {
	src := newSourceMock()
	src.listReposFunc = func(_ context.Context) ([]models.Repository, error) {
		return []models.Repository{{Name: "repo1"}}, nil
	}

	s, err := NewSyncer(src, []platform.Platform{}, testCredentials(), slog.Default())
	be.True(t, err == nil)
	defer s.Close()

	diff, err := s.ListDiff(context.Background())
	be.True(t, err == nil)
	be.Equal(t, len(diff), 0)
}

func TestListDiffSourceError(t *testing.T) {
	src := newSourceMock()
	src.listReposFunc = func(_ context.Context) ([]models.Repository, error) {
		return nil, platform.ErrNotAuthenticated
	}

	s, err := NewSyncer(src, []platform.Platform{newDestMock("gitlab")}, testCredentials(), slog.Default())
	be.True(t, err == nil)
	defer s.Close()

	_, err = s.ListDiff(context.Background())
	be.True(t, err != nil)
}

func TestListDiffDestError(t *testing.T) {
	src := newSourceMock()
	src.listReposFunc = func(_ context.Context) ([]models.Repository, error) {
		return []models.Repository{}, nil
	}

	dest := newDestMock("gitlab")
	dest.listReposFunc = func(_ context.Context) ([]models.Repository, error) {
		return nil, platform.ErrNotAuthenticated
	}

	s, err := NewSyncer(src, []platform.Platform{dest}, testCredentials(), slog.Default())
	be.True(t, err == nil)
	defer s.Close()

	_, err = s.ListDiff(context.Background())
	be.True(t, err != nil)
}

func TestSyncAllSourceListError(t *testing.T) {
	src := newSourceMock()
	src.listReposFunc = func(_ context.Context) ([]models.Repository, error) {
		return nil, platform.ErrNotAuthenticated
	}

	s, err := NewSyncer(src, []platform.Platform{newDestMock("gitlab")}, testCredentials(), slog.Default())
	be.True(t, err == nil)
	defer s.Close()

	_, err = s.SyncAll(context.Background())
	be.True(t, err != nil)
}

func TestSyncAllDestListError(t *testing.T) {
	src := newSourceMock()
	src.listReposFunc = func(_ context.Context) ([]models.Repository, error) {
		return []models.Repository{{Name: "repo1"}}, nil
	}

	dest := newDestMock("gitlab")
	dest.listReposFunc = func(_ context.Context) ([]models.Repository, error) {
		return nil, platform.ErrNotAuthenticated
	}

	s, err := NewSyncer(src, []platform.Platform{dest}, testCredentials(), slog.Default())
	be.True(t, err == nil)
	defer s.Close()

	_, err = s.SyncAll(context.Background())
	be.True(t, err != nil)
}

func TestListRepositories(t *testing.T) {
	expectedRepos := []models.Repository{
		{Name: "repo1", Private: false},
		{Name: "repo2", Private: true},
	}
	src := newSourceMock()
	src.listReposFunc = func(_ context.Context) ([]models.Repository, error) {
		return expectedRepos, nil
	}

	s, err := NewSyncer(src, []platform.Platform{newDestMock("gitlab")}, testCredentials(), slog.Default())
	be.True(t, err == nil)
	defer s.Close()

	repos, err := s.ListRepositories(context.Background())
	be.True(t, err == nil)
	be.Equal(t, len(repos), 2)
	be.Equal(t, repos[0].Name, "repo1")
	be.Equal(t, repos[1].Name, "repo2")
}

func TestListRepositoriesError(t *testing.T) {
	src := newSourceMock()
	src.listReposFunc = func(_ context.Context) ([]models.Repository, error) {
		return nil, platform.ErrNotAuthenticated
	}

	s, err := NewSyncer(src, []platform.Platform{newDestMock("gitlab")}, testCredentials(), slog.Default())
	be.True(t, err == nil)
	defer s.Close()

	_, err = s.ListRepositories(context.Background())
	be.True(t, err != nil)
}

func TestSyncOneRepoExistsError(t *testing.T) {
	src := newSourceMock()
	src.getAuthUserFunc = func(_ context.Context) (string, error) { return "srcuser", nil }
	src.getRepoFunc = func(_ context.Context, _, _ string) (*models.Repository, error) {
		return &models.Repository{Name: "repo1", Private: false}, nil
	}

	dest := newDestMock("gitlab")
	dest.getAuthUserFunc = func(_ context.Context) (string, error) { return "destuser", nil }
	dest.repoExistsFunc = func(_ context.Context, _, _ string) (bool, error) {
		return false, platform.ErrNotAuthenticated
	}

	s, err := NewSyncer(src, []platform.Platform{dest}, testCredentials(), slog.Default())
	be.True(t, err == nil)
	defer s.Close()

	err = s.Init(context.Background())
	be.True(t, err == nil)

	results, err := s.SyncOne(context.Background(), "repo1")
	be.True(t, err == nil)
	be.Equal(t, len(results), 1)
	be.Equal(t, results[0].Action, models.ActionSkip)
	be.True(t, results[0].Error != nil)
}

func TestSyncOneGetRepoError(t *testing.T) {
	src := newSourceMock()
	src.getAuthUserFunc = func(_ context.Context) (string, error) { return "srcuser", nil }
	src.getRepoFunc = func(_ context.Context, _, _ string) (*models.Repository, error) {
		return nil, platform.ErrRepositoryNotFound
	}

	s, err := NewSyncer(src, []platform.Platform{newDestMock("gitlab")}, testCredentials(), slog.Default())
	be.True(t, err == nil)
	defer s.Close()

	_, err = s.SyncOne(context.Background(), "nonexistent-repo")
	be.True(t, err != nil)
}

func TestSyncAllWithRepos(t *testing.T) {
	src := newSourceMock()
	src.listReposFunc = func(_ context.Context) ([]models.Repository, error) {
		return []models.Repository{
			{Name: "repo1", Private: false},
			{Name: "repo2", Private: true},
		}, nil
	}

	dest := newDestMock("gitlab")
	dest.listReposFunc = func(_ context.Context) ([]models.Repository, error) {
		return []models.Repository{}, nil
	}

	s, err := NewSyncer(src, []platform.Platform{dest}, testCredentials(), slog.Default())
	be.True(t, err == nil)
	defer s.Close()

	results, err := s.SyncAll(context.Background())
	be.True(t, err == nil)
	be.Equal(t, len(results), 2)
	be.Equal(t, results[0].RepoName, "repo1")
	be.Equal(t, results[1].RepoName, "repo2")
}