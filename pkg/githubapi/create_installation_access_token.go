package githubapi

import (
	"encoding/json"
	"fmt"
	"github.com/cockroachdb/errors"
	"io"
	"net/http"
	"time"
)

// AccessTokenResponse represents the response from GitHub API for access token
type AccessTokenResponse struct {
	Token               string            `json:"token"`
	ExpiresAt           time.Time         `json:"expires_at"`
	Permissions         map[string]string `json:"permissions"`
	RepositorySelection string            `json:"repository_selection"`
	Repositories        []Repository      `json:"repositories,omitempty"`
}

// Repository represents a GitHub repository
type Repository struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// CreateInstallationAccessToken creates an installation access token for a GitHub App
func (c *Client) CreateInstallationAccessToken(installationID string, jwt string) (*AccessTokenResponse, error) {
	url := fmt.Sprintf("%s/app/installations/%s/access_tokens", c.config.BaseURL, installationID)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return &AccessTokenResponse{}, errors.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	resp, err := c.http.Do(req)
	if err != nil {
		return &AccessTokenResponse{}, errors.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &AccessTokenResponse{}, errors.Errorf("error reading response body: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		return &AccessTokenResponse{}, errors.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}
	var tokenResp AccessTokenResponse
	err = json.Unmarshal(body, &tokenResp)
	if err != nil {
		return &AccessTokenResponse{}, errors.Errorf("error unmarshaling response: %v", err)
	}
	return &tokenResp, nil
}
