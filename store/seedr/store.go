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

// StoreClient implements Seedr as a cloud-drive store (like PikPak cloud).
// No magnets, no torrent lifecycle – only browse and stream existing files.
type StoreClient struct {
	Name   store.StoreName
	client *Client
}

func NewStoreClient(token string) *StoreClient {
	return &StoreClient{
		Name:   store.StoreName("seedr"),
		client: NewClient(token),
	}
}

func (s *StoreClient) GetName() store.StoreName {
	return s.Name
}

// ---------------- Locked link format ----------------
//
// We encode folderId:fileId similar to PikPak to avoid exposing raw IDs.
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

// ---------------- Helpers ----------------

func toSize(i int64) int64 {
	if i <= 0 {
		return -1
	}
	return i
}

// Recursively flatten all files under a folder
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

	source := string(s.GetName())

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

	for _, folder := range res.Folders {
		folderFile := &store.MagnetFile{
			Idx:    -1,
			Link:   "",
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

// ---------------- Core Store API ----------------

// Seedr has no magnets. We fake "magnets" as root folders.
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

	data := &store.ListMagnetsData{
		Items:      items[startIdx:endIdx],
		TotalItems: totalItems,
	}
	return data, nil
}

// Treat a folder as a "magnet"
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

	data := &store.GetMagnetData{
		Id:      params.Id,
		Name:    "Seedr Folder",
		Hash:    "",
		Size:    size,
		Status:  store.MagnetStatusDownloaded,
		Files:   files,
		AddedAt: time.Now(),
	}

	return data, nil
}

// Generate direct stream URL from Seedr
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

	data := &store.GenerateLinkData{
		Link: res.URL,
	}
	return data, nil
}

// ---------------- Utility ----------------

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}