package svc

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type Client struct {
	Host        string
	Port        int
	Username    string
	Password    string
	InsecureTLS bool
	Token       string
	TokenExpiry time.Time
	HTTPClient  *http.Client
	mu          sync.Mutex
}

const (
	defaultPort        = 7443
	defaultHTTPTimeout = 120 * time.Second
	fabricTimeout      = 300 * time.Second // Fabric operations can take longer
)

func NewClient(host, username, password string) *Client {
	return &Client{
		Host:     host,
		Username: username,
		Password: password,
		Port:     defaultPort,
		HTTPClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}
}

func (c *Client) WithPort(port int) {
	c.Port = port
}

func (c *Client) WithTLSInsecure() *Client {
	c.InsecureTLS = true
	
	// Clone the default transport to preserve connection pooling!
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	
	c.HTTPClient.Transport = customTransport
	return c
}

func (c *Client) WithHTTPTimeout(timeout time.Duration) {
	c.HTTPClient.Timeout = timeout

}

func (c *Client) baseURL() string {
	return fmt.Sprintf("https://%s:%d", c.Host, c.Port)
}

func (c *Client) authenticateLocked(ctx context.Context, httpClient *http.Client) error {
	url := fmt.Sprintf("%s/rest/auth", c.baseURL())

	// Use NewRequestWithContext here
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Username", c.Username)
	req.Header.Set("X-Auth-Password", c.Password)

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// Drain the response body to allow socket reuse
		io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("authentication failed: %s", resp.Status)
	}

	var data struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}

	c.Token = data.Token
	c.TokenExpiry = time.Now().Add(30 * time.Minute)
	return nil
}

func (c *Client) Authenticate(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.authenticateLocked(ctx, c.HTTPClient)
}

func (c *Client) ensureTokenValid(ctx context.Context, httpClient *http.Client) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Token == "" || time.Now().After(c.TokenExpiry.Add(-2*time.Minute)) {
		if err := c.authenticateLocked(ctx, httpClient); err != nil {
			return "", err
		}
	}
	return c.Token, nil // Safely return the token while locked
}

func (c *Client) post(ctx context.Context, endpoint string, payload map[string]interface{}) ([]byte, error) {
	return c.postWithHTTPClient(ctx, c.HTTPClient, endpoint, payload)
}

func (c *Client) postWithHTTPClient(ctx context.Context, httpClient *http.Client, endpoint string, payload map[string]interface{}) ([]byte, error) {
	activeToken, err := c.ensureTokenValid(ctx, httpClient)
	if err != nil {
		return nil, fmt.Errorf("token refresh failed: %v", err)
	}

	url := fmt.Sprintf("%s/rest/%s", c.baseURL(), endpoint)

	var body io.Reader
	if payload != nil {
		jsonBody, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", activeToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, errors.New(string(respBody))
	}

	return respBody, nil
}