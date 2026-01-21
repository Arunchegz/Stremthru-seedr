package seedr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type APIClient struct {
	BaseURL    *url.URL
	HTTPClient *http.Client
	Email      string
	Password   string
	UserAgent  string
}

type APIClientConfig struct {
	HTTPClient *http.Client
	Email      string
	Password   string
	UserAgent  string
}

func NewAPIClient(conf *APIClientConfig) *APIClient {
	baseURL, _ := url.Parse("https://www.seedr.cc")
	
	if conf.HTTPClient == nil {
		conf.HTTPClient = http.DefaultClient
	}
	
	if conf.UserAgent == "" {
		conf.UserAgent = "StremThru/1.0"
	}

	return &APIClient{
		BaseURL:    baseURL,
		HTTPClient: conf.HTTPClient,
		Email:      conf.Email,
		Password:   conf.Password,
		UserAgent:  conf.UserAgent,
	}
}

func (c *APIClient) makeRequest(method, path string, body io.Reader) (*http.Response, error) {
	u, err := url.Parse(c.BaseURL.String())
	if err != nil {
		return nil, err
	}
	u.Path = path

	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(c.Email, c.Password)
	req.Header.Set("User-Agent", c.UserAgent)
	
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	return c.HTTPClient.Do(req)
}

// GetSettings retrieves user account settings
func (c *APIClient) GetSettings() (*SettingsResponse, error) {
	resp, err := c.makeRequest("GET", "/rest/settings", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", resp.Status)
	}

	var settings SettingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&settings); err != nil {
		return nil, err
	}

	if !settings.Result {
		return nil, fmt.Errorf("API request failed")
	}

	return &settings, nil
}

// AddMagnet adds a magnet link
func (c *APIClient) AddMagnet(magnetLink string) (*AddMagnetResponse, error) {
	data := url.Values{}
	data.Set("magnet", magnetLink)

	resp, err := c.makeRequest("POST", "/rest/torrent/magnet", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", resp.Status)
	}

	var result AddMagnetResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.Result {
		return nil, fmt.Errorf("failed to add magnet")
	}

	return &result, nil
}

// AddTorrentFile uploads a torrent file
func (c *APIClient) AddTorrentFile(fileData []byte, filename string) (*AddMagnetResponse, error) {
	// Create multipart form data
	body := &bytes.Buffer{}
	// Note: Proper multipart implementation needed here
	// This is a simplified version
	
	resp, err := c.makeRequest("POST", "/rest/torrent/file", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result AddMagnetResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.Result {
		return nil, fmt.Errorf("failed to add torrent file")
	}

	return &result, nil
}

// GetFolder retrieves folder contents
func (c *APIClient) GetFolder(folderID *int) (*FolderResponse, error) {
	path := "/rest/folder"
	if folderID != nil {
		path = fmt.Sprintf("/rest/folder/%d", *folderID)
	}

	resp, err := c.makeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", resp.Status)
	}

	var folder FolderResponse
	if err := json.NewDecoder(resp.Body).Decode(&folder); err != nil {
		return nil, err
	}

	if !folder.Result {
		return nil, fmt.Errorf("failed to get folder")
	}

	return &folder, nil
}

// DeleteFolder deletes a folder
func (c *APIClient) DeleteFolder(folderID int) error {
	path := fmt.Sprintf("/rest/folder/%d", folderID)
	
	resp, err := c.makeRequest("DELETE", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var result DeleteFolderResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if !result.Result {
		return fmt.Errorf("failed to delete folder")
	}

	return nil
}

// DeleteFile deletes a file
func (c *APIClient) DeleteFile(fileID int) error {
	path := fmt.Sprintf("/rest/file/%d", fileID)
	
	resp, err := c.makeRequest("DELETE", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var result DeleteFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if !result.Result {
		return fmt.Errorf("failed to delete file")
	}

	return nil
}

// GetFileURL generates authenticated download URL for a file
func (c *APIClient) GetFileURL(fileID int) string {
	// Seedr files are accessed via direct URL with basic auth
	return fmt.Sprintf("https://%s:%s@www.seedr.cc/rest/file/%d", 
		url.QueryEscape(c.Email), 
		url.QueryEscape(c.Password), 
		fileID)
}

// ParseFolderID extracts folder ID from magnet metadata
func ParseFolderID(metadata string) (*int, error) {
	if metadata == "" {
		return nil, nil
	}
	id, err := strconv.Atoi(metadata)
	if err != nil {
		return nil, err
	}
	return &id, nil
}