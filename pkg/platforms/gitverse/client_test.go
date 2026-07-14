package gitverse

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

func newGitverseTestClient(serverURL string) *Client {
	return &Client{
		api:    apiclient.New(serverURL, "test-token", apiclient.Config{AuthHeader: "Authorization", AuthPrefix: "Bearer "}),
		webURL: "https://gitverse.ru",
	}
}

func TestGetAuthenticatedUser(t *testing.T) {
	for _, tc := range GVGetUserTestCases() {
		t.Run(tc.Name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.ResponseCode)
				if tc.ResponseBody != nil {
					var respBody []byte
					if errResp, ok := tc.ResponseBody.(GVErrorResponse); ok {
						respBody = MockGVErrorResponse(errResp.Message)
					} else if user, ok := tc.ResponseBody.(GVUserResponse); ok {
						respBody = MockGVUserResponse(user.Login)
					}
					w.Write(respBody)
				}
			}))
			defer server.Close()

			client := newGitverseTestClient(server.URL)

			user, err := client.GetAuthenticatedUser(context.Background())

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
	for _, tc := range GVListReposTestCases() {
		t.Run(tc.Name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.ResponseCode)
				if tc.ResponseBody != nil {
					var respBody []byte
					if errResp, ok := tc.ResponseBody.(GVErrorResponse); ok {
						respBody = MockGVErrorResponse(errResp.Message)
					} else if repos, ok := tc.ResponseBody.([]GVRepositoryResponse); ok {
						respBody = MockGVRepoListResponse(repos)
					}
					w.Write(respBody)
				}
			}))
			defer server.Close()

			client := newGitverseTestClient(server.URL)

			repos, err := client.ListRepositories(context.Background())

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
	for _, tc := range GVGetRepoTestCases() {
		t.Run(tc.Name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.ResponseCode)
				if tc.ResponseBody != nil {
					var respBody []byte
					if errResp, ok := tc.ResponseBody.(GVErrorResponse); ok {
						respBody = MockGVErrorResponse(errResp.Message)
					} else if repo, ok := tc.ResponseBody.(GVRepositoryResponse); ok {
						respBody = MockGVRepoResponse(repo)
					}
					w.Write(respBody)
				}
			}))
			defer server.Close()

			client := newGitverseTestClient(server.URL)

			repo, err := client.GetRepository(context.Background(), tc.Owner, tc.Repo)

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
	for _, tc := range GVRepoExistsTestCases() {
		t.Run(tc.Name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.ResponseCode)
			}))
			defer server.Close()

			client := newGitverseTestClient(server.URL)

			exists, err := client.RepositoryExists(context.Background(), tc.Owner, tc.Repo)

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
	for _, tc := range GVCreateRepoTestCases() {
		t.Run(tc.Name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				be.Equal(t, r.Method, "POST")
				be.Equal(t, r.URL.Path, "/user/repos")
				w.WriteHeader(tc.ResponseCode)
				if tc.ResponseBody != nil {
					var respBody []byte
					if errResp, ok := tc.ResponseBody.(GVErrorResponse); ok {
						respBody = MockGVErrorResponse(errResp.Message)
					} else if repo, ok := tc.ResponseBody.(GVRepositoryResponse); ok {
						respBody = MockGVRepoResponse(repo)
					}
					w.Write(respBody)
				}
			}))
			defer server.Close()

			client := newGitverseTestClient(server.URL)

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
		be.Equal(t, r.Method, "PATCH")
		be.True(t, len(r.URL.Path) > 0)
		w.WriteHeader(200)
	}))
	defer server.Close()

	client := newGitverseTestClient(server.URL)

	err := client.UpdateRepository(context.Background(), "user", "myrepo", true, "Updated description")
	be.Equal(t, err, nil)
}

func TestUpdateRepositoryError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"Not Found"}`)
	}))
	defer server.Close()

	client := newGitverseTestClient(server.URL)

	err := client.UpdateRepository(context.Background(), "user", "nonexistent", false, "")
	be.True(t, err != nil)
}

func TestCloneURL(t *testing.T) {
	client := &Client{webURL: "https://gitverse.ru"}
	repo := models.Repository{FullName: "user/repo"}

	url := client.CloneURL(repo)
	be.Equal(t, url, "https://gitverse.ru/user/repo.git")
}

func TestCleanPullRefs(t *testing.T) {
	client := &Client{}
	err := client.CleanPullRefs("/tmp/nonexistent")
	be.Equal(t, err, nil)
}

func TestConfigure(t *testing.T) {
	client := &Client{}
	err := client.Configure("token", "https://api.gitverse.ru", "https://gitverse.ru", "")
	be.Equal(t, err, nil)
	be.True(t, client.api != nil)
}

func TestID(t *testing.T) {
	client := &Client{}
	be.Equal(t, client.ID(), models.PlatformID("gitverse"))
}

func TestName(t *testing.T) {
	client := &Client{}
	be.Equal(t, client.Name(), "GitVerse")
}

func BenchmarkGetAuthenticatedUser(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write(MockGVUserResponse("testuser"))
	}))
	defer server.Close()

	client := newGitverseTestClient(server.URL)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.GetAuthenticatedUser(context.Background())
	}
}

func BenchmarkListRepositories(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write(MockGVRepoListResponse([]GVRepositoryResponse{
			{Name: "repo1", FullName: "user/repo1"},
		}))
	}))
	defer server.Close()

	client := newGitverseTestClient(server.URL)

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

	client := newGitverseTestClient(server.URL)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.RepositoryExists(context.Background(), "user", "repo")
	}
}