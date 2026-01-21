package seedr

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const baseURL = "https://www.seedr.cc/api"

type Client struct {
	Token      string
	HTTPClient *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		Token: token,
		HTTPClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (c *Client) doRequest(method, url string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	// Seedr uses Bearer token authentication
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")

	return c.HTTPClient.Do(req)
}

func (c *Client) getJSON(url string, v any) error {
	res, err := c.doRequest("GET", url)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("seedr api error: %s", res.Status)
	}

	return json.NewDecoder(res.Body).Decode(v)
}