package seedr

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/Arunchegz/Stremthru-seedr/store"
)

type StoreClient struct {
	Name       store.StoreName
	apiClient  *APIClient
}

type StoreClientConfig struct {
	HTTPClient *http.Client
	Email      string
	Password   string
	UserAgent  string
}

func NewStoreClient(config *StoreClientConfig) *StoreClient {
	apiClient := NewAPIClient(&APIClientConfig{
		HTTPClient: config.HTTPClient,
		Email:      config.Email,
		Password:   config.Password,
		UserAgent:  config.UserAgent,
	})

	return &StoreClient{
		Name:      store.StoreNameSeedr,
		apiClient: apiClient,
	}
}

func (c *StoreClient) GetName() store.StoreName {
	return c.Name
}

func (c *StoreClient) GetUser(params *store.GetUserParams) (*store.User, error) {
	settings, err := c.apiClient.GetSettings()
	if err != nil {
		return nil, err
	}

	subscriptionStatus := store.UserSubscriptionStatusExpired
	if settings.Settings.Account.IsPremium {
		subscriptionStatus = store.UserSubscriptionStatusPremium
	}

	return &store.User{
		ID:                 settings.Settings.Account.Username,
		Email:              settings.Settings.Account.Email,
		SubscriptionStatus: subscriptionStatus,
	}, nil
}

func (c *StoreClient) AddMagnet(params *store.AddMagnetParams) (*store.AddMagnetData, error) {
	var result *AddMagnetResponse
	var err error

	if params.Magnet != "" {
		result, err = c.apiClient.AddMagnet(params.Magnet)
	} else if len(params.Torrent) > 0 {
		result, err = c.apiClient.AddTorrentFile(params.Torrent, "torrent.torrent")
	} else {
		return nil, fmt.Errorf("either magnet or torrent file required")
	}

	if err != nil {
		return nil, err
	}

	// Wait a moment for processing to start
	time.Sleep(2 * time.Second)

	// Get folder contents to get file information
	folder, err := c.apiClient.GetFolder(nil)
	if err != nil {
		return nil, err
	}

	// Find the folder matching our torrent
	var targetFolder *Folder
	for _, f := range folder.Folders {
		if strings.Contains(strings.ToLower(f.Name), strings.ToLower(result.Title)) {
			targetFolder = &f
			break
		}
	}

	magnetData := &store.AddMagnetData{
		ID:      fmt.Sprintf("%d", result.UserTorrent),
		Hash:    result.TorrentHash,
		Magnet:  params.Magnet,
		Name:    result.Title,
		Status:  store.MagnetStatusQueued,
		AddedAt: time.Now(),
	}

	if targetFolder != nil {
		magnetData.Size = targetFolder.Size
		// Store folder ID in metadata for later retrieval
		magnetData.ID = fmt.Sprintf("%d", targetFolder.ID)
		
		// Check if download is complete
		folderContents, err := c.apiClient.GetFolder(&targetFolder.ID)
		if err == nil && len(folderContents.Files) > 0 {
			magnetData.Status = store.MagnetStatusDownloaded
			magnetData.Files = c.convertFiles(folderContents.Files, targetFolder.ID)
		}
	}

	return magnetData, nil
}

func (c *StoreClient) GetMagnet(params *store.GetMagnetParams) (*store.GetMagnetData, error) {
	folderID, err := ParseFolderID(params.MagnetId)
	if err != nil {
		return nil, fmt.Errorf("invalid magnet ID: %w", err)
	}

	folder, err := c.apiClient.GetFolder(folderID)
	if err != nil {
		return nil, err
	}

	status := store.MagnetStatusDownloaded
	if len(folder.Files) == 0 {
		status = store.MagnetStatusDownloading
	}

	files := c.convertFiles(folder.Files, folder.ID)

	return &store.GetMagnetData{
		ID:      params.MagnetId,
		Name:    folder.Name,
		Status:  status,
		Files:   files,
		AddedAt: time.Now(),
	}, nil
}

func (c *StoreClient) ListMagnets(params *store.ListMagnetsParams) (*store.ListMagnetsData, error) {
	folder, err := c.apiClient.GetFolder(nil)
	if err != nil {
		return nil, err
	}

	magnets := []store.ListMagnetsItem{}
	for _, f := range folder.Folders {
		magnets = append(magnets, store.ListMagnetsItem{
			ID:      fmt.Sprintf("%d", f.ID),
			Name:    f.Name,
			Size:    f.Size,
			Status:  store.MagnetStatusDownloaded,
			AddedAt: f.LastUpdate,
		})
	}

	// Apply limit and offset
	start := params.Offset
	end := start + params.Limit
	if start > len(magnets) {
		start = len(magnets)
	}
	if end > len(magnets) {
		end = len(magnets)
	}

	return &store.ListMagnetsData{
		Items:      magnets[start:end],
		TotalItems: len(magnets),
	}, nil
}

func (c *StoreClient) RemoveMagnet(params *store.RemoveMagnetParams) (*store.RemoveMagnetData, error) {
	folderID, err := ParseFolderID(params.MagnetId)
	if err != nil {
		return nil, fmt.Errorf("invalid magnet ID: %w", err)
	}

	if folderID == nil {
		return nil, fmt.Errorf("folder ID required")
	}

	if err := c.apiClient.DeleteFolder(*folderID); err != nil {
		return nil, err
	}

	return &store.RemoveMagnetData{
		ID: params.MagnetId,
	}, nil
}

func (c *StoreClient) CheckMagnet(params *store.CheckMagnetParams) (*store.CheckMagnetData, error) {
	// Seedr doesn't have a cache check endpoint
	// We return unknown status for all magnets
	items := []store.CheckMagnetItem{}
	
	for _, magnet := range params.Magnets {
		items = append(items, store.CheckMagnetItem{
			Magnet: magnet,
			Hash:   extractHashFromMagnet(magnet),
			Status: store.MagnetStatusUnknown,
		})
	}

	return &store.CheckMagnetData{
		Items: items,
	}, nil
}

func (c *StoreClient) GenerateLink(params *store.GenerateLinkParams) (*store.GenerateLinkData, error) {
	// For Seedr, links are already direct download URLs with auth
	// We can just return the same link with proper auth
	return &store.GenerateLinkData{
		Link: params.Link,
	}, nil
}

// Helper functions

func (c *StoreClient) convertFiles(files []File, folderID int) []store.MagnetFile {
	result := []store.MagnetFile{}
	
	for i, file := range files {
		// Generate authenticated URL
		fileURL := c.apiClient.GetFileURL(file.FolderFileID)
		
		result = append(result, store.MagnetFile{
			Index: i,
			Name:  file.Name,
			Path:  filepath.Join(fmt.Sprintf("%d", folderID), file.Name),
			Size:  file.Size,
			Link:  fileURL,
		})
	}
	
	return result
}

func extractHashFromMagnet(magnet string) string {
	// Extract hash from magnet link
	// Format: magnet:?xt=urn:btih:HASH
	if !strings.HasPrefix(magnet, "magnet:") {
		return ""
	}
	
	parts := strings.Split(magnet, "xt=urn:btih:")
	if len(parts) < 2 {
		return ""
	}
	
	hash := strings.Split(parts[1], "&")[0]
	return strings.ToLower(hash)
}