package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/vutran1710/dating-dev/internal/cli/config"
)

type APIClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewAPIClient(cfg *config.Config) *APIClient {
	return &APIClient{
		baseURL:    cfg.Server.BackendURL,
		token:      cfg.Auth.Token,
		httpClient: &http.Client{},
	}
}

func (c *APIClient) Get(path string) (*http.Response, error) {
	return c.do("GET", path, nil)
}

func (c *APIClient) Post(path string, body any) (*http.Response, error) {
	return c.do("POST", path, body)
}

func (c *APIClient) Put(path string, body any) (*http.Response, error) {
	return c.do("PUT", path, body)
}

func (c *APIClient) do(method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request: %w", err)
		}
		bodyReader = strings.NewReader(string(data))
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	return c.httpClient.Do(req)
}

func DecodeResponse[T any](resp *http.Response) (*T, error) {
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &result, nil
}
