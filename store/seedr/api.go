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

func (c *Client) doRequest(url string, v any) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("seedr api error: %s", res.Status)
	}

	return json.NewDecoder(res.Body).Decode(v)
}

func (c *Client) GetFolders() (*FoldersResponse, error) {
	var resp FoldersResponse
	err := c.doRequest(baseURL+"/folders", &resp)
	return &resp, err
}

func (c *Client) GetFolder(folderID int) (*FolderResponse, error) {
	var resp FolderResponse
	err := c.doRequest(fmt.Sprintf("%s/folder/%d", baseURL, folderID), &resp)
	return &resp, err
}

func (c *Client) GetFile(fileID int) (*FileResponse, error) {
	var resp FileResponse
	err := c.doRequest(fmt.Sprintf("%s/file/%d", baseURL, fileID), &resp)
	return &resp, err
}