package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gh-mirror/pkg/apiclient"
	"gh-mirror/pkg/models"
	"gh-mirror/pkg/platform"
)

const PlatformID = models.PlatformID("gitlab")

type Client struct {
	api    *apiclient.Client
	webURL string
	owner  string
}

func init() {
	platform.Register(PlatformID, func() platform.Platform {
		return &Client{}
	})
}

func (c *Client) ID() models.PlatformID {
	return PlatformID
}

func (c *Client) Name() string {
	return "GitLab"
}

func (c *Client) Configure(token string, apiURL string, webURL string, owner string) error {
	c.api = apiclient.New(strings.TrimSuffix(apiURL, "/"), token, apiclient.Config{
		AuthHeader: "PRIVATE-TOKEN",
		AuthPrefix: "",
	})
	c.webURL = webURL
	c.owner = owner
	return nil
}

func (c *Client) GetAuthenticatedUser(ctx context.Context) (string, error) {
	resp, err := c.api.DoRequest(ctx, "GET", "/user", nil)
	if err != nil {
		return "", err
	}

	var user struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(resp, &user); err != nil {
		return "", fmt.Errorf("parse user response: %w", err)
	}

	return user.Username, nil
}

func (c *Client) ListRepositories(ctx context.Context) ([]models.Repository, error) {
	var allRepos []models.Repository
	page := 1
	perPage := 100

	for {
		path := fmt.Sprintf("/projects?membership=true&page=%d&per_page=%d", page, perPage)
		resp, err := c.api.DoRequest(ctx, "GET", path, nil)
		if err != nil {
			return nil, fmt.Errorf("list repositories: %w", err)
		}

		var repos []struct {
			ID                int64  `json:"id"`
			Name              string `json:"name"`
			PathWithNameSpace string `json:"path_with_namespace"`
			Description       string `json:"description"`
			Visibility        string `json:"visibility"`
			WebURL            string `json:"web_url"`
			DefaultBranch     string `json:"default_branch"`
		}
		if err := json.Unmarshal(resp, &repos); err != nil {
			return nil, fmt.Errorf("parse repos response: %w", err)
		}

		for _, r := range repos {
			allRepos = append(allRepos, models.Repository{
				PlatformID:    PlatformID,
				Name:          r.Name,
				FullName:      r.PathWithNameSpace,
				Description:   r.Description,
				Private:       r.Visibility == "private",
				HTMLURL:       r.WebURL,
				DefaultBranch: r.DefaultBranch,
			})
		}

		if len(repos) < perPage {
			break
		}
		page++
	}

	return allRepos, nil
}

func (c *Client) GetRepository(ctx context.Context, owner, repo string) (*models.Repository, error) {
	path := fmt.Sprintf("/projects/%s%%2F%s", owner, repo)
	resp, err := c.api.DoRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var r struct {
		ID                int64  `json:"id"`
		Name              string `json:"name"`
		PathWithNameSpace string `json:"path_with_namespace"`
		Description       string `json:"description"`
		Visibility        string `json:"visibility"`
		WebURL            string `json:"web_url"`
		DefaultBranch     string `json:"default_branch"`
	}
	if err := json.Unmarshal(resp, &r); err != nil {
		return nil, fmt.Errorf("parse repo response: %w", err)
	}

	return &models.Repository{
		PlatformID:    PlatformID,
		Name:          r.Name,
		FullName:      r.PathWithNameSpace,
		Description:   r.Description,
		Private:       r.Visibility == "private",
		HTMLURL:       r.WebURL,
		DefaultBranch: r.DefaultBranch,
	}, nil
}

func (c *Client) CreateRepository(ctx context.Context, name string, private bool, description string) (*models.Repository, error) {
	visibility := "public"
	if private {
		visibility = "private"
	}

	reqBody := struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Visibility  string `json:"visibility"`
	}{
		Name:        name,
		Description: description,
		Visibility:  visibility,
	}

	resp, err := c.api.DoRequest(ctx, "POST", "/projects", reqBody)
	if err != nil {
		return nil, fmt.Errorf("create repository: %w", err)
	}

	var r struct {
		ID                int64  `json:"id"`
		Name              string `json:"name"`
		PathWithNameSpace string `json:"path_with_namespace"`
		Description       string `json:"description"`
		Visibility        string `json:"visibility"`
		WebURL            string `json:"web_url"`
		DefaultBranch     string `json:"default_branch"`
	}
	if err := json.Unmarshal(resp, &r); err != nil {
		return nil, fmt.Errorf("parse create response: %w", err)
	}

	return &models.Repository{
		PlatformID:    PlatformID,
		Name:          r.Name,
		FullName:      r.PathWithNameSpace,
		Description:   r.Description,
		Private:       r.Visibility == "private",
		HTMLURL:       r.WebURL,
		DefaultBranch: r.DefaultBranch,
	}, nil
}

func (c *Client) UpdateRepository(ctx context.Context, owner, repo string, private bool, description string) error {
	path := fmt.Sprintf("/projects/%s%%2F%s", owner, repo)
	visibility := "public"
	if private {
		visibility = "private"
	}
	reqBody := struct {
		Description string `json:"description,omitempty"`
		Visibility  string `json:"visibility"`
	}{
		Description: description,
		Visibility:  visibility,
	}

	_, err := c.api.DoRequest(ctx, "PUT", path, reqBody)
	if err != nil {
		return fmt.Errorf("update repository: %w", err)
	}

	return nil
}

func (c *Client) RepositoryExists(ctx context.Context, owner, repo string) (bool, error) {
	path := fmt.Sprintf("/projects/%s%%2F%s", owner, repo)
	_, err := c.api.DoRequest(ctx, "GET", path, nil)
	if err != nil {
		if apiErr, ok := err.(*apiclient.APIError); ok && apiErr.StatusCode == 404 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *Client) CloneURL(repo models.Repository) string {
	return fmt.Sprintf("%s/%s.git", c.webURL, repo.FullName)
}

func (c *Client) CleanPullRefs(repoPath string) error {
	return nil
}