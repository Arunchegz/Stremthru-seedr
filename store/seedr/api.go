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
// Account info (optional, useful to test token)
func (c *Client) GetAccount() (*AccountResponse, error) {
	var resp AccountResponse
	err := c.getJSON(baseURL+"/account", &resp)
	return &resp, err
}

// List root folders
func (c *Client) GetFolders() (*FoldersResponse, error) {
	var resp FoldersResponse
	err := c.getJSON(baseURL+"/folders", &resp)
	return &resp, err
}

// List files inside a folder
func (c *Client) GetFolder(folderID int) (*FolderResponse, error) {
	var resp FolderResponse
	err := c.getJSON(fmt.Sprintf("%s/folder/%d", baseURL, folderID), &resp)
	return &resp, err
}

// Get direct stream/download URL for a file
func (c *Client) GetFile(fileID int) (*FileResponse, error) {
	var resp FileResponse
	err := c.getJSON(fmt.Sprintf("%s/file/%d", baseURL, fileID), &resp)
	return &resp, err
}