package svc

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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
}

const (
	defaultPort        = 7443
	defaultHTTPTimeout = 10 * time.Second
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
	c.HTTPClient.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return c
}

func (c *Client) WithHTTPTimeout(timeout time.Duration) {
	c.HTTPClient.Timeout = timeout

}

func (c *Client) baseURL() string {
	return fmt.Sprintf("https://%s:%d", c.Host, c.Port)
}

func (c *Client) Authenticate() error {
	url := fmt.Sprintf("%s/rest/auth", c.baseURL())

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Username", c.Username)
	req.Header.Set("X-Auth-Password", c.Password)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
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

func (c *Client) ensureTokenValid() error {
	if c.Token == "" || time.Now().After(c.TokenExpiry.Add(-2*time.Minute)) {
		return c.Authenticate()
	}
	return nil
}

func (c *Client) post(endpoint string, payload map[string]interface{}) ([]byte, error) {
	if err := c.ensureTokenValid(); err != nil {
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

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, errors.New(string(respBody))
	}

	return io.ReadAll(resp.Body)
}
