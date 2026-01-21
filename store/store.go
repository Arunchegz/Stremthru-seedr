package seedr

import (
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/MunifTanjim/stremthru/core"
	"github.com/MunifTanjim/stremthru/store"
)

/*
Seedr Cloud Store
=================
Cloud-only implementation:
- List folders as "magnets"
- Open folder → list files
- GenerateLink → direct Seedr CDN URL

NO:
- AddMagnet
- RemoveMagnet
- CheckMagnet
- Torrent logic
*/

type StoreClient struct {
	Name   store.StoreName
	client *Client
}

func NewStoreClient(token string) *StoreClient {
	return &StoreClient{
		Name:   store.StoreNameSeedr,
		client: NewClient(token),
	}
}

func (s *StoreClient) GetName() store.StoreName {
	return s.Name
}

/* ---------------- Locked File Link ---------------- */

type LockedFileLink string

const lockedFileLinkPrefix = "stremthru://store/seedr/"

func (l LockedFileLink) encodeData(folderId, fileId string) string {
	return core.Base64Encode(folderId + ":" + fileId)
}

func (l LockedFileLink) decodeData(encoded string) (folderId, fileId string, err error) {
	decoded, err := core.Base64Decode(encoded)
	if err != nil {
		return "", "", err
	}
	folderId, fileId, found := strings.Cut(decoded, ":")
	if !found {
		return "", "", err
	}
	return folderId, fileId, nil
}

func (l LockedFileLink) create(folderId, fileId string) string {
	return lockedFileLinkPrefix + l.encodeData(folderId, fileId)
}

func (l LockedFileLink) parse() (folderId, fileId string, err error) {
	encoded := strings.TrimPrefix(string(l), lockedFileLinkPrefix)
	return l.decodeData(encoded)
}

/* ---------------- Helpers ---------------- */

func toSize(size int64) int64 {
	if size <= 0 {
		return -1
	}
	return size
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Recursively flatten files in a folder tree
func (s *StoreClient) listFilesFlat(
	folderID int,
	result []store.MagnetFile,
	parent *store.MagnetFile,
	rootFolderID int,
) ([]store.MagnetFile, error) {

	if result == nil {
		result = []store.MagnetFile{}
	}

	res, err := s.client.GetFolder(folderID)
	if err != nil {
		return nil, err
	}

	source := string(s.GetName().Code())

	// Files
	for _, f := range res.Files {
		file := &store.MagnetFile{
			Idx:    -1,
			Link:   LockedFileLink("").create(strconv.Itoa(rootFolderID), strconv.Itoa(f.Id)),
			Name:   f.Name,
			Path:   "/" + f.Name,
			Size:   toSize(f.Size),
			Source: source,
		}

		if parent != nil {
			file.Path = path.Join(parent.Path, file.Name)
		}

		result = append(result, *file)
	}

	// Subfolders
	for _, folder := range res.Folders {
		folderFile := &store.MagnetFile{
			Idx:    -1,
			Name:   folder.Name,
			Path:   "/" + folder.Name,
			Size:   toSize(folder.Size),
			Source: source,
		}

		if parent != nil {
			folderFile.Path = path.Join(parent.Path, folderFile.Name)
		}

		result, err = s.listFilesFlat(folder.Id, result, folderFile, rootFolderID)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

/* ---------------- Store Interface ---------------- */

// List all Seedr folders as "magnets"
func (s *StoreClient) ListMagnets(params *store.ListMagnetsParams) (*store.ListMagnetsData, error) {
	res, err := s.client.GetFolders()
	if err != nil {
		return nil, err
	}

	items := []store.ListMagnetsDataItem{}

	for _, folder := range res.Folders {
		item := store.ListMagnetsDataItem{
			Id:      strconv.Itoa(folder.Id),
			Name:    folder.Name,
			Hash:    "",
			Size:    toSize(folder.Size),
			Status:  store.MagnetStatusDownloaded,
			AddedAt: time.Now(),
		}
		items = append(items, item)
	}

	totalItems := len(items)
	startIdx := min(params.Offset, totalItems)
	endIdx := min(startIdx+params.Limit, totalItems)

	return &store.ListMagnetsData{
		Items:      items[startIdx:endIdx],
		TotalItems: totalItems,
	}, nil
}

// Open a Seedr folder
func (s *StoreClient) GetMagnet(params *store.GetMagnetParams) (*store.GetMagnetData, error) {
	folderID, err := strconv.Atoi(params.Id)
	if err != nil {
		return nil, err
	}

	files, err := s.listFilesFlat(folderID, nil, nil, folderID)
	if err != nil {
		return nil, err
	}

	var size int64 = 0
	for i := range files {
		size += files[i].Size
	}

	return &store.GetMagnetData{
		Id:      params.Id,
		Name:    "Seedr Folder",
		Hash:    "",
		Size:    size,
		Status:  store.MagnetStatusDownloaded,
		Files:   files,
		AddedAt: time.Now(),
	}, nil
}

// Generate direct streaming URL
func (s *StoreClient) GenerateLink(params *store.GenerateLinkParams) (*store.GenerateLinkData, error) {
	_, fileId, err := LockedFileLink(params.Link).parse()
	if err != nil {
		e := core.NewAPIError("invalid link")
		e.StatusCode = http.StatusBadRequest
		return nil, e
	}

	id, err := strconv.Atoi(fileId)
	if err != nil {
		return nil, err
	}

	res, err := s.client.GetFile(id)
	if err != nil {
		return nil, err
	}

	return &store.GenerateLinkData{
		Link: res.URL,
	}, nil
}

/* ---------------- Unsupported Methods ---------------- */

func (s *StoreClient) AddMagnet(params *store.AddMagnetParams) (*store.AddMagnetData, error) {
	return nil, core.NewStoreError("Seedr is cloud-only, AddMagnet is not supported")
}

func (s *StoreClient) RemoveMagnet(params *store.RemoveMagnetParams) (*store.RemoveMagnetData, error) {
	return nil, core.NewStoreError("Seedr is cloud-only, RemoveMagnet is not supported")
}

func (s *StoreClient) CheckMagnet(params *store.CheckMagnetParams) (*store.CheckMagnetData, error) {
	return nil, core.NewStoreError("Seedr is cloud-only, CheckMagnet is not supported")
}

func (s *StoreClient) GetUser(params *store.GetUserParams) (*store.User, error) {
	return &store.User{
		Id:                 "seedr",
		Email:              "",
		SubscriptionStatus: store.UserSubscriptionStatusPremium,
	}, nil
}