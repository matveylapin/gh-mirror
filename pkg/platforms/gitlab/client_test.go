package gitlab

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nalgeon/be"
	"gh-mirror/pkg/apiclient"
	"gh-mirror/pkg/models"
)

func newTestClient(serverURL string) *Client {
	return &Client{
		api:    apiclient.New(serverURL, "test-token", apiclient.Config{AuthHeader: "PRIVATE-TOKEN", AuthPrefix: ""}),
		webURL: "https://gitlab.com",
	}
}

func TestGetAuthenticatedUser(t *testing.T) {
	for _, tc := range GetAuthenticatedUserTestCases() {
		t.Run(tc.Name, func(t *testing.T) {
			var receivedPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedPath = r.URL.Path
				w.WriteHeader(tc.ResponseCode)
				if tc.ResponseBody != nil {
					var respBody []byte
					if str, ok := tc.ResponseBody.(APIErrorResponse); ok {
						respBody = MockErrorResponse(str.Message)
					} else if user, ok := tc.ResponseBody.(UserResponse); ok {
						respBody = MockUserResponse(user.Username)
					}
					w.Write(respBody)
				}
			}))
			defer server.Close()

			client := newTestClient(server.URL)

			user, err := client.GetAuthenticatedUser(context.Background())
			be.Equal(t, receivedPath, "/user")

			if tc.WantErr {
				be.True(t, err != nil)
			} else {
				be.Equal(t, err, nil)
				be.Equal(t, user, tc.WantUser)
			}
		})
	}
}

func TestListRepositories(t *testing.T) {
	for _, tc := range ListRepositoriesTestCases() {
		t.Run(tc.Name, func(t *testing.T) {
			var receivedPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedPath = r.URL.Path
				w.WriteHeader(tc.ResponseCode)
				if tc.ResponseBody != nil {
					var respBody []byte
					if errResp, ok := tc.ResponseBody.(APIErrorResponse); ok {
						respBody = MockErrorResponse(errResp.Message)
					} else if repos, ok := tc.ResponseBody.([]RepositoryResponse); ok {
						respBody = MockRepositoryListResponse(repos)
					}
					w.Write(respBody)
				}
			}))
			defer server.Close()

			client := newTestClient(server.URL)

			repos, err := client.ListRepositories(context.Background())
			be.True(t, len(receivedPath) > 0)

			if tc.WantErr {
				be.True(t, err != nil)
			} else {
				be.Equal(t, err, nil)
				be.Equal(t, len(repos), tc.WantCount)
			}
		})
	}
}

func TestGetRepository(t *testing.T) {
	for _, tc := range GetRepositoryTestCases() {
		t.Run(tc.Name, func(t *testing.T) {
			var receivedPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedPath = r.URL.Path
				w.WriteHeader(tc.ResponseCode)
				if tc.ResponseBody != nil {
					var respBody []byte
					if errResp, ok := tc.ResponseBody.(APIErrorResponse); ok {
						respBody = MockErrorResponse(errResp.Message)
					} else if repo, ok := tc.ResponseBody.(RepositoryResponse); ok {
						respBody = MockRepositoryResponse(repo)
					}
					w.Write(respBody)
				}
			}))
			defer server.Close()

			client := newTestClient(server.URL)

			repo, err := client.GetRepository(context.Background(), tc.Owner, tc.Repo)
			be.True(t, len(receivedPath) > 0)

			if tc.WantErr {
				be.True(t, err != nil)
			} else {
				be.Equal(t, err, nil)
				be.Equal(t, repo.Name, tc.WantRepoName)
			}
		})
	}
}

func TestRepositoryExists(t *testing.T) {
	for _, tc := range RepositoryExistsTestCases() {
		t.Run(tc.Name, func(t *testing.T) {
			var receivedPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedPath = r.URL.Path
				w.WriteHeader(tc.ResponseCode)
			}))
			defer server.Close()

			client := newTestClient(server.URL)

			exists, err := client.RepositoryExists(context.Background(), tc.Owner, tc.Repo)
			be.True(t, len(receivedPath) > 0)

			if tc.WantErr {
				be.True(t, err != nil)
			} else {
				be.Equal(t, err, nil)
				be.Equal(t, exists, tc.WantExists)
			}
		})
	}
}

func TestCreateRepository(t *testing.T) {
	for _, tc := range CreateRepositoryTestCases() {
		t.Run(tc.Name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				be.Equal(t, r.Method, "POST")
				be.Equal(t, r.URL.Path, "/projects")
				w.WriteHeader(tc.ResponseCode)
				if tc.ResponseBody != nil {
					var respBody []byte
					if errResp, ok := tc.ResponseBody.(APIErrorResponse); ok {
						respBody = MockErrorResponse(errResp.Message)
					} else if repo, ok := tc.ResponseBody.(RepositoryResponse); ok {
						respBody = MockRepositoryResponse(repo)
					}
					w.Write(respBody)
				}
			}))
			defer server.Close()

			client := newTestClient(server.URL)

			repo, err := client.CreateRepository(context.Background(), "new-repo", false, "A new repo")

			if tc.WantErr {
				be.True(t, err != nil)
			} else {
				be.Equal(t, err, nil)
				be.Equal(t, repo.Name, tc.WantRepoName)
			}
		})
	}
}

func TestUpdateRepository(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		be.Equal(t, r.Method, "PUT")
		be.True(t, len(r.URL.Path) > 0)
		w.WriteHeader(200)
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	err := client.UpdateRepository(context.Background(), "user", "myrepo", true, "Updated description")
	be.Equal(t, err, nil)
}

func TestUpdateRepositoryError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"404 Project Not Found"}`)
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	err := client.UpdateRepository(context.Background(), "user", "nonexistent", false, "")
	be.True(t, err != nil)
}

func TestCloneURL(t *testing.T) {
	client := &Client{webURL: "https://gitlab.com"}
	repo := models.Repository{FullName: "user/repo"}

	url := client.CloneURL(repo)
	be.Equal(t, url, "https://gitlab.com/user/repo.git")
}

func TestCleanPullRefs(t *testing.T) {
	client := &Client{}
	err := client.CleanPullRefs("/tmp/nonexistent")
	be.Equal(t, err, nil)
}

func TestConfigure(t *testing.T) {
	client := &Client{}
	err := client.Configure("token", "https://gitlab.com/api/v4", "https://gitlab.com", "")
	be.Equal(t, err, nil)
	be.True(t, client.api != nil)
}

func TestID(t *testing.T) {
	client := &Client{}
	be.Equal(t, client.ID(), models.PlatformID("gitlab"))
}

func TestName(t *testing.T) {
	client := &Client{}
	be.Equal(t, client.Name(), "GitLab")
}

func BenchmarkGetAuthenticatedUser(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write(MockUserResponse("testuser"))
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.GetAuthenticatedUser(context.Background())
	}
}

func BenchmarkListRepositories(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write(MockRepositoryListResponse([]RepositoryResponse{
			{ID: 1, Name: "repo1", PathWithNameSpace: "user/repo1", Visibility: "public"},
		}))
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.ListRepositories(context.Background())
	}
}

func BenchmarkRepositoryExists(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.RepositoryExists(context.Background(), "user", "repo")
	}
}