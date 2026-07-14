package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/nalgeon/be"

	gh "github.com/google/go-github/v67/github"
	"gh-mirror/pkg/models"
)

func TestGetAuthenticatedUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		be.Equal(t, r.URL.Path, "/user")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"login":"testuser"}`)
	}))
	defer server.Close()

	client := newClientWithServer(server)
	user, err := client.GetAuthenticatedUser(context.Background())
	be.Equal(t, err, nil)
	be.Equal(t, user, "testuser")
}

func TestGetAuthenticatedUserError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"message":"Bad credentials"}`)
	}))
	defer server.Close()

	client := newClientWithServer(server)
	_, err := client.GetAuthenticatedUser(context.Background())
	be.True(t, err != nil)
}

func TestListRepositories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		be.True(t, r.URL.Path == "/user/repos")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{
			"name": "test-repo",
			"full_name": "testuser/test-repo",
			"description": "A test repo",
			"private": false,
			"html_url": "https://github.com/testuser/test-repo",
			"default_branch": "main"
		}]`)
	}))
	defer server.Close()

	client := newClientWithServer(server)
	repos, err := client.ListRepositories(context.Background())
	be.Equal(t, err, nil)
	be.Equal(t, len(repos), 1)
	be.Equal(t, repos[0].Name, "test-repo")
	be.Equal(t, repos[0].FullName, "testuser/test-repo")
	be.Equal(t, repos[0].Private, false)
}

func TestListRepositoriesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := newClientWithServer(server)
	_, err := client.ListRepositories(context.Background())
	be.True(t, err != nil)
}

func TestGetRepository(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		be.Equal(t, r.URL.Path, "/repos/testuser/test-repo")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"name": "test-repo",
			"full_name": "testuser/test-repo",
			"description": "A test repo",
			"private": true,
			"html_url": "https://github.com/testuser/test-repo",
			"default_branch": "main"
		}`)
	}))
	defer server.Close()

	client := newClientWithServer(server)
	repo, err := client.GetRepository(context.Background(), "testuser", "test-repo")
	be.Equal(t, err, nil)
	be.Equal(t, repo.Name, "test-repo")
	be.Equal(t, repo.Private, true)
}

func TestGetRepositoryNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"Not Found"}`)
	}))
	defer server.Close()

	client := newClientWithServer(server)
	_, err := client.GetRepository(context.Background(), "testuser", "nonexistent")
	be.True(t, err != nil)
}

func TestCreateRepository(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		be.Equal(t, r.Method, "POST")
		be.Equal(t, r.URL.Path, "/user/repos")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"name": "new-repo",
			"full_name": "testuser/new-repo",
			"description": "New repo",
			"private": true,
			"html_url": "https://github.com/testuser/new-repo",
			"default_branch": "main"
		}`)
	}))
	defer server.Close()

	client := newClientWithServer(server)
	repo, err := client.CreateRepository(context.Background(), "new-repo", true, "New repo")
	be.Equal(t, err, nil)
	be.Equal(t, repo.Name, "new-repo")
	be.Equal(t, repo.Private, true)
}

func TestCreateRepositoryError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprint(w, `{"message":"Validation Failed"}`)
	}))
	defer server.Close()

	client := newClientWithServer(server)
	_, err := client.CreateRepository(context.Background(), "existing-repo", false, "")
	be.True(t, err != nil)
}

func TestUpdateRepository(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		be.Equal(t, r.Method, "PATCH")
		be.Equal(t, r.URL.Path, "/repos/testuser/test-repo")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"name": "test-repo",
			"full_name": "testuser/test-repo",
			"private": true,
			"description": "Updated description"
		}`)
	}))
	defer server.Close()

	client := newClientWithServer(server)
	err := client.UpdateRepository(context.Background(), "testuser", "test-repo", true, "Updated description")
	be.Equal(t, err, nil)
}

func TestUpdateRepositoryError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"Not Found"}`)
	}))
	defer server.Close()

	client := newClientWithServer(server)
	err := client.UpdateRepository(context.Background(), "testuser", "nonexistent", false, "")
	be.True(t, err != nil)
}

func TestRepositoryExists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		be.Equal(t, r.URL.Path, "/repos/testuser/test-repo")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"name": "test-repo"}`)
	}))
	defer server.Close()

	client := newClientWithServer(server)
	exists, err := client.RepositoryExists(context.Background(), "testuser", "test-repo")
	be.Equal(t, err, nil)
	be.Equal(t, exists, true)
}

func TestRepositoryNotExists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"Not Found"}`)
	}))
	defer server.Close()

	client := newClientWithServer(server)
	exists, err := client.RepositoryExists(context.Background(), "testuser", "nonexistent")
	be.Equal(t, err, nil)
	be.Equal(t, exists, false)
}

func TestRepositoryExistsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := newClientWithServer(server)
	_, err := client.RepositoryExists(context.Background(), "testuser", "test-repo")
	be.True(t, err != nil)
}

func TestCloneURL(t *testing.T) {
	for _, tc := range GHCloneURLTestCases() {
		t.Run(tc.Name, func(t *testing.T) {
			client := &Client{webURL: tc.WebURL}
			repo := models.Repository{FullName: tc.FullName}

			url := client.CloneURL(repo)
			be.Equal(t, url, tc.WantURL)
		})
	}
}

func TestConfigure(t *testing.T) {
	for _, tc := range GHConfigureTestCases() {
		t.Run(tc.Name, func(t *testing.T) {
			client := &Client{}
			err := client.Configure(tc.Token, tc.APIURL, tc.WebURL, "")

			if tc.WantErr {
				be.True(t, err != nil)
			} else {
				be.Equal(t, err, nil)
				be.Equal(t, client.token, tc.Token)
				be.Equal(t, client.webURL, tc.WebURL)
			}
		})
	}
}

func TestConfigureRequiresWebURL(t *testing.T) {
	client := &Client{}
	err := client.Configure("token", "", "", "")
	be.True(t, err != nil)
}

func TestID(t *testing.T) {
	client := &Client{}
	be.Equal(t, client.ID(), models.PlatformID("github"))
}

func TestName(t *testing.T) {
	client := &Client{}
	be.Equal(t, client.Name(), "GitHub")
}

func TestCleanPullRefsNonExistent(t *testing.T) {
	client := &Client{}
	err := client.CleanPullRefs("/tmp/nonexistent-path-12345")
	be.True(t, err != nil)
}

func TestCleanPullRefsInvalidPath(t *testing.T) {
	client := &Client{}
	err := client.CleanPullRefs("/invalid/path/that/cannot/be/opened")
	be.True(t, err != nil)
}

func TestCleanPullRefsRemovesPullRefs(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := tmpDir

	repo, err := git.PlainInit(repoPath, false)
	be.True(t, err == nil)

	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{"https://github.com/user/repo.git"},
	})
	be.True(t, err == nil)

	wt, err := repo.Worktree()
	be.True(t, err == nil)

	err = os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("test"), 0644)
	be.True(t, err == nil)
	_, err = wt.Add("README.md")
	be.True(t, err == nil)
	_, err = wt.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@test.com"},
	})
	be.True(t, err == nil)

	err = repo.Storer.SetReference(plumbing.NewHashReference("refs/heads/main", plumbing.NewHash("abc123def456abc123def456abc123def456abcd")))
	be.True(t, err == nil)
	err = repo.Storer.SetReference(plumbing.NewHashReference("refs/pull/1/head", plumbing.NewHash("def456abc123def456abc123def456abc123def4")))
	be.True(t, err == nil)
	err = repo.Storer.SetReference(plumbing.NewHashReference("refs/pull/2/head", plumbing.NewHash("123456789abc123456789abc123456789abc1234")))
	be.True(t, err == nil)

	client := &Client{}
	err = client.CleanPullRefs(repoPath)
	be.True(t, err == nil)

	refs, err := repo.References()
	be.True(t, err == nil)

	pullRefCount := 0
	refs.ForEach(func(ref *plumbing.Reference) error {
		if strings.HasPrefix(ref.Name().String(), "refs/pull/") {
			pullRefCount++
		}
		return nil
	})
	be.Equal(t, pullRefCount, 0)
}

func TestCleanPullRefsNoPullRefs(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := tmpDir

	_, err := git.PlainInit(repoPath, false)
	be.True(t, err == nil)

	client := &Client{}
	err = client.CleanPullRefs(repoPath)
	be.True(t, err == nil)
}

func newClientWithServer(server *httptest.Server) *Client {
	ghClient := gh.NewClient(server.Client())
	ghClient.BaseURL, _ = ghClient.BaseURL.Parse(server.URL + "/")
	return &Client{
		token:    "test-token",
		webURL:   "https://github.com",
		client:   ghClient,
	}
}

func BenchmarkCloneURL(b *testing.B) {
	client := &Client{webURL: "https://github.com"}
	repo := models.Repository{FullName: "user/repo"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.CloneURL(repo)
	}
}

func BenchmarkConfigure(b *testing.B) {
	client := &Client{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.Configure("token", "https://api.github.com", "https://github.com", "")
	}
}

func BenchmarkID(b *testing.B) {
	client := &Client{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.ID()
	}
}

func BenchmarkName(b *testing.B) {
	client := &Client{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.Name()
	}
}