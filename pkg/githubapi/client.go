package githubapi

import "net/http"

// Client represents a GitHub API client
type Client struct {
	config ClientConfig
	http   *http.Client
}

// ClientConfig holds the configuration for the GitHub API client
type ClientConfig struct {
	BaseURL string
}

// NewClient creates a new GitHub API client
func NewClient(config ClientConfig) *Client {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.github.com"
	}
	return &Client{
		config: config,
		http:   &http.Client{},
	}
}
