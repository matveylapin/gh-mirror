package codeberg

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gh-mirror/pkg/apiclient"
	"gh-mirror/pkg/models"
	"gh-mirror/pkg/platform"
)

const PlatformID = models.PlatformID("codeberg")

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
	return "Codeberg"
}

func (c *Client) Configure(token string, apiURL string, webURL string, owner string) error {
	c.api = apiclient.New(strings.TrimSuffix(apiURL, "/"), token, apiclient.Config{
		AuthHeader: "Authorization",
		AuthPrefix: "token ",
	})
	c.webURL = strings.TrimSuffix(webURL, "/")
	c.owner = owner
	return nil
}

func (c *Client) GetAuthenticatedUser(ctx context.Context) (string, error) {
	resp, err := c.api.DoRequest(ctx, "GET", "/user", nil)
	if err != nil {
		return "", err
	}

	var user struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal(resp, &user); err != nil {
		return "", fmt.Errorf("parse user response: %w", err)
	}

	return user.Login, nil
}

func (c *Client) ListRepositories(ctx context.Context) ([]models.Repository, error) {
	var allRepos []models.Repository
	page := 1
	limit := 50

	for {
		path := fmt.Sprintf("/user/repos?page=%d&limit=%d", page, limit)
		resp, err := c.api.DoRequest(ctx, "GET", path, nil)
		if err != nil {
			return nil, fmt.Errorf("list repositories: %w", err)
		}

		var repos []struct {
			ID            int64  `json:"id"`
			Name          string `json:"name"`
			FullName      string `json:"full_name"`
			Description   string `json:"description"`
			Private       bool   `json:"private"`
			HTMLURL       string `json:"html_url"`
			CloneURL      string `json:"clone_url"`
			DefaultBranch string `json:"default_branch"`
		}
		if err := json.Unmarshal(resp, &repos); err != nil {
			return nil, fmt.Errorf("parse repos response: %w", err)
		}

		if len(repos) == 0 {
			break
		}

		for _, r := range repos {
			allRepos = append(allRepos, models.Repository{
				PlatformID:    PlatformID,
				Name:          r.Name,
				FullName:      r.FullName,
				Description:   r.Description,
				Private:       r.Private,
				HTMLURL:       r.HTMLURL,
				DefaultBranch: r.DefaultBranch,
			})
		}

		if len(repos) < limit {
			break
		}
		page++
	}

	return allRepos, nil
}

func (c *Client) GetRepository(ctx context.Context, owner, repo string) (*models.Repository, error) {
	path := fmt.Sprintf("/repos/%s/%s", owner, repo)
	resp, err := c.api.DoRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var r struct {
		ID            int64  `json:"id"`
		Name          string `json:"name"`
		FullName      string `json:"full_name"`
		Description   string `json:"description"`
		Private       bool   `json:"private"`
		HTMLURL       string `json:"html_url"`
		CloneURL      string `json:"clone_url"`
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.Unmarshal(resp, &r); err != nil {
		return nil, fmt.Errorf("parse repo response: %w", err)
	}

	return &models.Repository{
		PlatformID:    PlatformID,
		Name:          r.Name,
		FullName:      r.FullName,
		Description:   r.Description,
		Private:       r.Private,
		HTMLURL:       r.HTMLURL,
		DefaultBranch: r.DefaultBranch,
	}, nil
}

func (c *Client) CreateRepository(ctx context.Context, name string, private bool, description string) (*models.Repository, error) {
	reqBody := struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Private     bool   `json:"private"`
	}{
		Name:        name,
		Description: description,
		Private:     private,
	}

	resp, err := c.api.DoRequest(ctx, "POST", "/user/repos", reqBody)
	if err != nil {
		return nil, fmt.Errorf("create repository: %w", err)
	}

	var r struct {
		ID            int64  `json:"id"`
		Name          string `json:"name"`
		FullName      string `json:"full_name"`
		Description   string `json:"description"`
		Private       bool   `json:"private"`
		HTMLURL       string `json:"html_url"`
		CloneURL      string `json:"clone_url"`
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.Unmarshal(resp, &r); err != nil {
		return nil, fmt.Errorf("parse create response: %w", err)
	}

	return &models.Repository{
		PlatformID:    PlatformID,
		Name:          r.Name,
		FullName:      r.FullName,
		Description:   r.Description,
		Private:       r.Private,
		HTMLURL:       r.HTMLURL,
		DefaultBranch: r.DefaultBranch,
	}, nil
}

func (c *Client) UpdateRepository(ctx context.Context, owner, repo string, private bool, description string) error {
	path := fmt.Sprintf("/repos/%s/%s", owner, repo)
	reqBody := struct {
		Description string `json:"description,omitempty"`
		Private     bool   `json:"private"`
	}{
		Description: description,
		Private:     private,
	}

	_, err := c.api.DoRequest(ctx, "PATCH", path, reqBody)
	if err != nil {
		return fmt.Errorf("update repository: %w", err)
	}

	return nil
}

func (c *Client) RepositoryExists(ctx context.Context, owner, repo string) (bool, error) {
	path := fmt.Sprintf("/repos/%s/%s", owner, repo)
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
	if c.owner != "" {
		return fmt.Sprintf("%s/%s/%s.git", c.webURL, c.owner, repo.Name)
	}
	return fmt.Sprintf("%s/%s.git", c.webURL, repo.FullName)
}

func (c *Client) CleanPullRefs(repoPath string) error {
	return nil
}