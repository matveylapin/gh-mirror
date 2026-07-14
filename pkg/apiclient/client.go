package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	maxRetries    = 3
	maxBackoff    = 10 * time.Second
	requestTimeout = 60 * time.Second
	maxErrorBody  = 1024
)

type Config struct {
	AuthHeader   string
	AuthPrefix   string
	ExtraHeaders map[string]string
}

type Client struct {
	apiURL     string
	token      string
	httpClient *http.Client
	cfg        Config
}

func New(apiURL, token string, cfg Config) *Client {
	return NewWithHTTPClient(apiURL, token, cfg, &http.Client{
		Timeout: requestTimeout,
	})
}

func NewWithHTTPClient(apiURL, token string, cfg Config, httpClient *http.Client) *Client {
	return &Client{
		apiURL:     apiURL,
		token:      token,
		httpClient: httpClient,
		cfg:        cfg,
	}
}

func (c *Client) DoRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
	}

	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		var reqBody io.Reader
		if bodyBytes != nil {
			reqBody = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequest(method, c.apiURL+path, reqBody)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		if c.cfg.AuthHeader != "" {
			authValue := c.cfg.AuthPrefix + c.token
			req.Header.Set(c.cfg.AuthHeader, authValue)
		}
		req.Header.Set("Content-Type", "application/json")
		for key, value := range c.cfg.ExtraHeaders {
			req.Header.Set(key, value)
		}

		resp, err := c.httpClient.Do(req.WithContext(ctx))
		if err != nil {
			lastErr = fmt.Errorf("execute request: %w", err)
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read response body: %w", err)
			continue
		}

		if resp.StatusCode < 400 {
			return respBody, nil
		}

		if resp.StatusCode < 500 && resp.StatusCode != 429 {
			return nil, &APIError{
				StatusCode: resp.StatusCode,
				Message:    TruncateBody(respBody),
			}
		}

		lastErr = &APIError{
			StatusCode: resp.StatusCode,
			Message:    TruncateBody(respBody),
		}
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", maxRetries, lastErr)
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error: status=%d, body=%s", e.StatusCode, e.Message)
}

func TruncateBody(body []byte) string {
	msg := string(body)
	if len(msg) > maxErrorBody {
		msg = msg[:maxErrorBody]
	}
	return msg
}