package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gh-mirror/pkg/models"
	"gh-mirror/pkg/platform"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/go-github/v67/github"
)

const PlatformID = models.PlatformID("github")

type Client struct {
	token  string
	webURL string
	owner  string
	client *github.Client
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
	return "GitHub"
}

func (c *Client) Configure(token string, apiURL string, webURL string, owner string) error {
	if webURL == "" {
		return fmt.Errorf("web URL is required")
	}
	c.token = token
	c.webURL = webURL
	c.owner = owner

	httpClient := &http.Client{Timeout: 60 * time.Second}

	if apiURL == "" {
		c.client = github.NewClient(httpClient).WithAuthToken(token)
	} else {
		c.client = github.NewClient(httpClient).WithAuthToken(token)
		baseURL := strings.TrimSuffix(apiURL, "/")
		parsedURL, err := url.Parse(baseURL + "/")
		if err != nil {
			return fmt.Errorf("invalid API URL: %w", err)
		}
		c.client.BaseURL = parsedURL
	}

	return nil
}

func (c *Client) GetAuthenticatedUser(ctx context.Context) (string, error) {
	user, _, err := c.client.Users.Get(ctx, "")
	if err != nil {
		return "", fmt.Errorf("get authenticated user: %w", err)
	}
	return user.GetLogin(), nil
}

func (c *Client) ListRepositories(ctx context.Context) ([]models.Repository, error) {
	if c.owner != "" {
		return c.listOrgRepositories(ctx)
	}
	return c.listUserRepositories(ctx)
}

func (c *Client) listOrgRepositories(ctx context.Context) ([]models.Repository, error) {
	var allRepos []models.Repository
	page := 1
	perPage := 100

	for {
		repos, resp, err := c.client.Repositories.ListByOrg(ctx, c.owner, &github.RepositoryListByOrgOptions{
			Sort:        "updated",
			Direction:   "desc",
			ListOptions: github.ListOptions{
				Page:    page,
				PerPage: perPage,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("list org repositories: %w", err)
		}

		for _, r := range repos {
			allRepos = append(allRepos, models.Repository{
				PlatformID:    PlatformID,
				Name:          r.GetName(),
				FullName:      r.GetFullName(),
				Description:   r.GetDescription(),
				Private:       r.GetPrivate(),
				HTMLURL:       r.GetHTMLURL(),
				DefaultBranch: r.GetDefaultBranch(),
				UpdatedAt:     r.GetUpdatedAt().String(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	return allRepos, nil
}

func (c *Client) listUserRepositories(ctx context.Context) ([]models.Repository, error) {
	var allRepos []models.Repository
	page := 1
	perPage := 100

	for {
		repos, resp, err := c.client.Repositories.List(ctx, "", &github.RepositoryListOptions{
			Type:      "owner",
			Sort:      "updated",
			Direction: "desc",
			ListOptions: github.ListOptions{
				Page:    page,
				PerPage: perPage,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("list repositories: %w", err)
		}

		for _, r := range repos {
			allRepos = append(allRepos, models.Repository{
				PlatformID:    PlatformID,
				Name:          r.GetName(),
				FullName:      r.GetFullName(),
				Description:   r.GetDescription(),
				Private:       r.GetPrivate(),
				HTMLURL:       r.GetHTMLURL(),
				DefaultBranch: r.GetDefaultBranch(),
				UpdatedAt:     r.GetUpdatedAt().String(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	return allRepos, nil
}

func (c *Client) GetRepository(ctx context.Context, owner, repo string) (*models.Repository, error) {
	r, _, err := c.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("get repository %s/%s: %w", owner, repo, err)
	}

	return &models.Repository{
		PlatformID:    PlatformID,
		Name:          r.GetName(),
		FullName:      r.GetFullName(),
		Description:   r.GetDescription(),
		Private:       r.GetPrivate(),
		HTMLURL:       r.GetHTMLURL(),
		DefaultBranch: r.GetDefaultBranch(),
		UpdatedAt:     r.GetUpdatedAt().String(),
	}, nil
}

func (c *Client) CreateRepository(ctx context.Context, name string, private bool, description string) (*models.Repository, error) {
	r, _, err := c.client.Repositories.Create(ctx, "", &github.Repository{
		Name:        &name,
		Private:     &private,
		Description: &description,
	})
	if err != nil {
		return nil, fmt.Errorf("create repository: %w", err)
	}

	return &models.Repository{
		PlatformID:    PlatformID,
		Name:          r.GetName(),
		FullName:      r.GetFullName(),
		Description:   r.GetDescription(),
		Private:       r.GetPrivate(),
		HTMLURL:       r.GetHTMLURL(),
		DefaultBranch: r.GetDefaultBranch(),
		UpdatedAt:     r.GetUpdatedAt().String(),
	}, nil
}

func (c *Client) UpdateRepository(ctx context.Context, owner, repo string, private bool, description string) error {
	_, _, err := c.client.Repositories.Edit(ctx, owner, repo, &github.Repository{
		Private:     &private,
		Description: &description,
	})
	if err != nil {
		return fmt.Errorf("update repository: %w", err)
	}
	return nil
}

func (c *Client) RepositoryExists(ctx context.Context, owner, repo string) (bool, error) {
	_, resp, err := c.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
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
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("open repo: %w", err)
	}

	refs, err := r.References()
	if err != nil {
		return fmt.Errorf("list references: %w", err)
	}

	var pullRefs []*plumbing.Reference
	refs.ForEach(func(ref *plumbing.Reference) error {
		if strings.HasPrefix(ref.Name().String(), "refs/pull/") {
			pullRefs = append(pullRefs, ref)
		}
		return nil
	})

	for _, ref := range pullRefs {
		if err := r.Storer.RemoveReference(ref.Name()); err != nil {
			return fmt.Errorf("remove reference %s: %w", ref.Name(), err)
		}
	}

	return nil
}
