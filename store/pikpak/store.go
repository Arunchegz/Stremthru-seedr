package pikpak

import (
	"errors"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/MunifTanjim/stremthru/core"
	"github.com/MunifTanjim/stremthru/internal/buddy"
	"github.com/MunifTanjim/stremthru/internal/cache"
	"github.com/MunifTanjim/stremthru/store"
)

func toSize(sizeStr string) int64 {
	size, err := strconv.Atoi(sizeStr)
	if err != nil {
		size = -1
	}
	return int64(size)
}

type StoreClientConfig struct {
	HTTPClient *http.Client
	UserAgent  string
}

type StoreClient struct {
	Name             store.StoreName
	client           *APIClient
	listMagnetsCache cache.Cache[[]store.ListMagnetsDataItem]
}

func NewStoreClient(config *StoreClientConfig) *StoreClient {
	c := &StoreClient{}
	c.client = NewAPIClient(&APIClientConfig{
		HTTPClient: config.HTTPClient,
		UserAgent:  config.UserAgent,
	})
	c.Name = store.StoreNamePikPak

	c.listMagnetsCache = func() cache.Cache[[]store.ListMagnetsDataItem] {
		return cache.NewCache[[]store.ListMagnetsDataItem](&cache.CacheConfig{
			Name:     "store:pikpak:listMagnets",
			Lifetime: 5 * time.Minute,
		})
	}()

	return c
}

func (s *StoreClient) getCacheKey(ctx Ctx, key string) string {
	return ctx.GetDeviceId() + ":" + key
}

func (s *StoreClient) GetName() store.StoreName {
	return s.Name
}

func (s *StoreClient) getRecentTask(ctx Ctx, taskId string) (*Task, error) {
	res, err := s.client.ListTasks(&ListTasksParams{
		Ctx:   ctx,
		Limit: 200,
		Filters: map[string]map[string]any{
			"phase": {
				"in": FilePhaseRunning + "," + FilePhaseError + "," + FilePhaseComplete,
			},
		},
	})
	if err != nil {
		return nil, err
	}
	for i := range res.Data.Tasks {
		t := &res.Data.Tasks[i]
		if t.Id == taskId {
			return t, nil
		}
	}
	error := core.NewStoreError("task not found: " + string(taskId))
	error.StoreName = string(s.GetName())
	return nil, error
}

func (s *StoreClient) waitForTaskComplete(ctx Ctx, taskId string, maxRetry int, retryInterval time.Duration) (*Task, error) {
	t, err := s.getRecentTask(ctx, taskId)
	if err != nil {
		return nil, err
	}
	retry := 0
	for (t.Phase != FilePhaseComplete && t.Phase != FilePhaseError) && retry < maxRetry {
		time.Sleep(retryInterval)
		task, err := s.getRecentTask(ctx, t.Id)
		if err != nil {
			return t, err
		}
		t = task
		retry++
	}
	if t.Phase != FilePhaseComplete {
		error := core.NewStoreError("task failed to reach phase: " + string(FilePhaseComplete))
		error.StoreName = string(s.GetName())
		return t, error
	}
	return t, nil
}

func (s *StoreClient) getFileByMagnetHash(ctx Ctx, hash string) (*File, error) {
	myPackFolder, err := s.getMyPackFolder(ctx)
	if err != nil {
		return nil, err
	}

	res, err := s.client.ListFiles(&ListFilesParams{
		Ctx:      ctx,
		Limit:    500,
		ParentId: myPackFolder.Id,
		Filters: map[string]map[string]any{
			"trashed": {"eq": false},
			"phase":   {"eq": FilePhaseComplete},
		},
	})
	if err != nil {
		return nil, err
	}
	for i := range res.Data.Files {
		f := &res.Data.Files[i]
		if strings.Contains(f.Params.URL, hash) {
			return f, nil
		}
	}
	return nil, nil
}

func (s *StoreClient) AddMagnet(params *store.AddMagnetParams) (*store.AddMagnetData, error) {
	if params.Magnet == "" {
		return nil, errors.New("torrent file not supported")
	}

	magnet, err := core.ParseMagnetLink(params.Magnet)
	if err != nil {
		return nil, err
	}
	ctx := Ctx{Ctx: params.Ctx}

	file, err := s.getFileByMagnetHash(ctx, magnet.Hash)
	if err != nil {
		return nil, err
	}

	data := &store.AddMagnetData{
		Hash:    magnet.Hash,
		Magnet:  magnet.Link,
		Name:    "",
		Size:    -1,
		Status:  store.MagnetStatusQueued,
		Files:   []store.MagnetFile{},
		AddedAt: time.Now(),
	}

	if file != nil {
		data.Id = file.Id

		mRes, err := s.GetMagnet(&store.GetMagnetParams{
			Ctx: ctx.Ctx,
			Id:  data.Id,
		})
		if err != nil {
			return nil, err
		}
		data.Name = mRes.Name
		data.Status = mRes.Status
		data.Files = mRes.Files
		data.AddedAt = mRes.AddedAt
		return data, nil
	}

	res, err := s.client.AddFile(&AddFileParams{
		Ctx: ctx,
		URL: AddFileParamsURL{
			URL: magnet.RawLink,
		},
	})
	if err != nil {
		return nil, err
	}

	s.listMagnetsCache.Remove(s.getCacheKey(ctx, ""))

	data.Id = res.Data.Task.FileId
	if task, err := s.waitForTaskComplete(ctx, res.Data.Task.Id, 3, 5*time.Second); task != nil {
		if err != nil {
			log.Error("error waiting for task complete", "error", err)
		}
		if task.Phase == FilePhaseComplete {
			mRes, err := s.GetMagnet(&store.GetMagnetParams{
				Ctx: ctx.Ctx,
				Id:  data.Id,
			})
			if err != nil {
				return nil, err
			}
			data.Name = mRes.Name
			data.Status = mRes.Status
			data.Files = mRes.Files
			data.AddedAt = mRes.AddedAt
		} else if task.Phase == FilePhaseError {
			data.Status = store.MagnetStatusFailed
		}
	}
	return data, nil
}

func (s *StoreClient) CheckMagnet(params *store.CheckMagnetParams) (*store.CheckMagnetData, error) {
	hashes := []string{}
	for _, m := range params.Magnets {
		magnet, err := core.ParseMagnetLink(m)
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, magnet.Hash)
	}

	data, err := buddy.CheckMagnet(s, hashes, params.GetAPIKey(s.client.apiKey), params.ClientIP, params.SId)
	if err != nil {
		return nil, err
	}
	return data, nil
}

type LockedFileLink string

const lockedFileLinkPrefix = "stremthru://store/pikpak/"

func (l LockedFileLink) encodeData(rootFileId, fileId string) string {
	return core.Base64Encode(rootFileId + ":" + fileId)
}

func (l LockedFileLink) decodeData(encoded string) (rootFileId, fileId string, err error) {
	decoded, err := core.Base64Decode(encoded)
	if err != nil {
		return "", "", err
	}
	rootFileId, fileId, found := strings.Cut(decoded, ":")
	if !found {
		return "", "", err
	}
	return rootFileId, fileId, nil
}

func (l LockedFileLink) create(rootFileId, fileId string) string {
	return lockedFileLinkPrefix + l.encodeData(rootFileId, fileId)
}

func (l LockedFileLink) parse() (rootFileId, fileId string, err error) {
	encoded := strings.TrimPrefix(string(l), lockedFileLinkPrefix)
	return l.decodeData(encoded)
}

func (s *StoreClient) GenerateLink(params *store.GenerateLinkParams) (*store.GenerateLinkData, error) {
	_, fileId, err := LockedFileLink(params.Link).parse()
	if err != nil {
		error := core.NewAPIError("invalid link")
		error.StoreName = string(s.GetName())
		error.StatusCode = http.StatusBadRequest
		error.Cause = err
		return nil, error
	}
	ctx := Ctx{Ctx: params.Ctx}
	res, err := s.client.GetFile(&GetFileParams{
		Ctx:    ctx,
		FileId: fileId,
	})
	if err != nil {
		return nil, err
	}
	if len(res.Data.Medias) == 0 {
		err := core.NewStoreError("file not found")
		err.StoreName = string(s.GetName())
		err.StatusCode = http.StatusNotFound
		return nil, err
	}
	data := &store.GenerateLinkData{
		Link: res.Data.Medias[0].Link.URL,
	}
	return data, nil
}

func (c *StoreClient) listFilesFlat(ctx Ctx, folderId string, result []store.MagnetFile, parent *store.MagnetFile, rootFolderId string) ([]store.MagnetFile, error) {
	if result == nil {
		result = []store.MagnetFile{}
	}

	params := &ListFilesParams{
		Ctx:      ctx,
		ParentId: folderId,
		Filters: map[string]map[string]any{
			"trashed": {"eq": false},
			"phase":   {"eq": FilePhaseComplete},
		},
	}
	lfRes, err := c.client.ListFiles(params)
	if err != nil {
		return nil, err
	}

	source := string(c.GetName().Code())
	for _, f := range lfRes.Data.Files {
		file := &store.MagnetFile{
			Idx:    -1, // order is non-deterministic
			Link:   LockedFileLink("").create(rootFolderId, f.Id),
			Name:   f.Name,
			Path:   "/" + f.Name,
			Size:   toSize(f.Size),
			Source: source,
		}

		if parent != nil {
			file.Path = path.Join(parent.Path, file.Name)
		}

		if f.Kind == FileKindFolder {
			result, err = c.listFilesFlat(ctx, f.Id, result, file, rootFolderId)
			if err != nil {
				return nil, err
			}
		} else {
			result = append(result, *file)
		}
	}

	return result, nil
}

func (s *StoreClient) GetMagnet(params *store.GetMagnetParams) (*store.GetMagnetData, error) {
	ctx := Ctx{Ctx: params.Ctx}
	res, err := s.client.GetFile(&GetFileParams{
		Ctx:    ctx,
		FileId: params.Id,
	})
	if err != nil {
		return nil, err
	}
	magnet, err := core.ParseMagnetLink(res.Data.Params.URL)
	if err != nil {
		return nil, err
	}
	addedAt, err := time.Parse(time.RFC3339, res.Data.CreatedTime)
	if err != nil {
		addedAt = time.Unix(0, 0)
	}
	data := &store.GetMagnetData{
		Id:      res.Data.Id,
		Name:    res.Data.Name,
		Hash:    magnet.Hash,
		Size:    -1,
		Status:  store.MagnetStatusDownloading,
		Files:   []store.MagnetFile{},
		AddedAt: addedAt,
	}
	if res.Data.Phase == FilePhaseComplete {
		data.Status = store.MagnetStatusDownloaded
		if res.Data.Kind == FileKindFolder {
			files, err := s.listFilesFlat(ctx, data.Id, nil, nil, data.Id)
			if err != nil {
				return nil, err
			}
			data.Files = files
			data.Size = 0
			for i := range files {
				data.Size += files[i].Size
			}
		} else {
			data.Files = append(data.Files, store.MagnetFile{
				Idx:    -1,
				Link:   LockedFileLink("").create(data.Id, data.Id),
				Name:   data.Name,
				Path:   "/" + data.Name,
				Size:   toSize(res.Data.Size),
				Source: string(s.GetName().Code()),
			})
		}
	}
	return data, nil
}

func (s *StoreClient) GetUser(params *store.GetUserParams) (*store.User, error) {
	res, err := s.client.GetUser(&GetUserParams{
		Ctx: Ctx{Ctx: params.Ctx},
	})
	if err != nil {
		return nil, err
	}
	vipRes, err := s.client.GetVIPInfo(&GetVIPInfoParams{
		Ctx: Ctx{Ctx: params.Ctx},
	})
	if err != nil {
		return nil, err
	}
	data := &store.User{
		Id:                 res.Data.Sub,
		Email:              res.Data.Email,
		SubscriptionStatus: store.UserSubscriptionStatusTrial,
	}
	if vipRes.Data.Type == VIPTypePlatinum {
		data.SubscriptionStatus = store.UserSubscriptionStatusPremium
	}
	return data, nil
}

func (s *StoreClient) getMyPackFolder(ctx Ctx) (*File, error) {
	res, err := s.client.ListFiles(&ListFilesParams{
		Ctx: ctx,
		Filters: map[string]map[string]any{
			"trashed": {"eq": false},
			"phase":   {"eq": FilePhaseComplete},
		},
	})
	if err != nil {
		return nil, err
	}
	for i := range res.Data.Files {
		f := &res.Data.Files[i]
		if f.Name == "My Pack" {
			return f, nil
		}
	}
	err = core.NewAPIError("'My Pack' folder missing")
	return nil, err
}

func (s *StoreClient) ListMagnets(params *store.ListMagnetsParams) (*store.ListMagnetsData, error) {
	ctx := Ctx{Ctx: params.Ctx}

	lm := []store.ListMagnetsDataItem{}

	if !s.listMagnetsCache.Get(s.getCacheKey(ctx, ""), &lm) {
		items := []store.ListMagnetsDataItem{}
		pageToken := ""
		for {
			myPackFolder, err := s.getMyPackFolder(ctx)
			if err != nil {
				return nil, err
			}
			res, err := s.client.ListFiles(&ListFilesParams{
				Ctx:      Ctx{Ctx: params.Ctx},
				Limit:    500,
				ParentId: myPackFolder.Id,
				Filters: map[string]map[string]any{
					"trashed": {"eq": false},
					"phase":   {"eq": FilePhaseComplete},
				},
				PageToken: pageToken,
			})
			if err != nil {
				return nil, err
			}

			for i := range res.Data.Files {
				f := &res.Data.Files[i]
				addedAt, err := time.Parse(time.RFC3339, f.CreatedTime)
				if err != nil {
					addedAt = time.Unix(0, 0)
				}
				if !strings.HasPrefix(f.Params.URL, "magnet:") {
					continue
				}
				magnet, err := core.ParseMagnetLink(f.Params.URL)
				if err != nil {
					continue
				}
				item := store.ListMagnetsDataItem{
					Id:      f.Id,
					Name:    f.Name,
					Hash:    magnet.Hash,
					Size:    toSize(f.Size),
					Status:  store.MagnetStatusDownloading,
					AddedAt: addedAt,
				}
				if f.Phase == FilePhaseComplete {
					item.Status = store.MagnetStatusDownloaded
				}
				items = append(items, item)
			}

			pageToken = res.Data.NextPageToken
			if pageToken == "" {
				break
			}
		}

		slices.SortFunc(items, func(a, b store.ListMagnetsDataItem) int {
			return b.AddedAt.Compare(a.AddedAt)
		})

		lm = items
		s.listMagnetsCache.Add(s.getCacheKey(ctx, ""), items)
	}

	totalItems := len(lm)
	startIdx := min(params.Offset, totalItems)
	endIdx := min(startIdx+params.Limit, totalItems)
	items := lm[startIdx:endIdx]

	data := &store.ListMagnetsData{
		Items:      items,
		TotalItems: totalItems,
	}

	return data, nil
}

func (s *StoreClient) RemoveMagnet(params *store.RemoveMagnetParams) (*store.RemoveMagnetData, error) {
	ctx := Ctx{Ctx: params.Ctx}
	_, err := s.client.Trash(&TrashParams{
		Ctx: ctx,
		Ids: []string{params.Id},
	})
	if err != nil {
		return nil, err
	}

	s.listMagnetsCache.Remove(s.getCacheKey(ctx, ""))

	data := &store.RemoveMagnetData{
		Id: params.Id,
	}
	return data, nil
}

// ============================================================================
// NEW METHODS FOR LISTING ALL FILES (NOT JUST MAGNETS)
// ============================================================================

// ListAllFiles lists all files in a directory (not limited to magnets)
func (s *StoreClient) ListAllFiles(params *store.ListAllFilesParams) (*store.ListAllFilesData, error) {
	ctx := Ctx{Ctx: params.Ctx}
	
	// Set defaults
	if params.PageSize == 0 {
		params.PageSize = 100
	}
	
	parentId := params.ParentID
	if parentId == "" {
		// If no parent specified, list root directory
		parentId = ""
	}
	
	// Build filters
	filters := map[string]map[string]any{
		"trashed": {"eq": false},
	}
	
	// Apply custom filters if provided
	if params.Filters != nil {
		if params.Filters.Kind != "" {
			if params.Filters.Kind == "folder" {
				filters["kind"] = map[string]any{"eq": FileKindFolder}
			} else if params.Filters.Kind == "file" {
				filters["kind"] = map[string]any{"eq": FileKindFile}
			}
		}
		if params.Filters.MimeType != "" {
			filters["mime_type"] = map[string]any{"eq": params.Filters.MimeType}
		}
	}
	
	// Call the API
	res, err := s.client.ListFiles(&ListFilesParams{
		Ctx:       ctx,
		ParentId:  parentId,
		Limit:     params.PageSize,
		PageToken: params.PageToken,
		Filters:   filters,
	})
	if err != nil {
		return nil, err
	}
	
	// Convert to store.FileItem format
	files := make([]store.FileItem, 0, len(res.Data.Files))
	for _, f := range res.Data.Files {
		fileType := store.FileTypeFile
		if f.Kind == FileKindFolder {
			fileType = store.FileTypeFolder
		}
		
		createdAt, _ := time.Parse(time.RFC3339, f.CreatedTime)
		modifiedAt, _ := time.Parse(time.RFC3339, f.ModifiedTime)
		
		fileItem := store.FileItem{
			ID:         f.Id,
			Name:       f.Name,
			Size:       toSize(f.Size),
			Type:       fileType,
			MimeType:   f.MimeType,
			CreatedAt:  createdAt,
			ModifiedAt: modifiedAt,
			ParentID:   f.ParentId,
			Hash:       f.Hash,
		}
		
		// Add download link if available
		if len(f.Medias) > 0 && f.Medias[0].Link.URL != "" {
			fileItem.DownloadLink = f.Medias[0].Link.URL
		}
		
		files = append(files, fileItem)
	}
	
	data := &store.ListAllFilesData{
		Files:         files,
		NextPageToken: res.Data.NextPageToken,
		HasMore:       res.Data.NextPageToken != "",
	}
	
	return data, nil
}

// ListAllFilesRecursive lists all files recursively from a parent folder
func (s *StoreClient) ListAllFilesRecursive(params *store.ListAllFilesRecursiveParams) ([]store.FileItem, error) {
	ctx := Ctx{Ctx: params.Ctx}
	allFiles := []store.FileItem{}
	
	var listFolder func(string) error
	listFolder = func(folderID string) error {
		pageToken := ""
		
		for {
			listParams := &store.ListAllFilesParams{
				Ctx:       params.Ctx,
				ParentID:  folderID,
				PageSize:  100,
				PageToken: pageToken,
			}
			
			data, err := s.ListAllFiles(listParams)
			if err != nil {
				return err
			}
			
			for _, file := range data.Files {
				allFiles = append(allFiles, file)
				
				// If it's a folder, recursively list its contents
				if file.Type == store.FileTypeFolder {
					if err := listFolder(file.ID); err != nil {
						return err
					}
				}
			}
			
			if !data.HasMore {
				break
			}
			pageToken = data.NextPageToken
		}
		
		return nil
	}
	
	if err := listFolder(params.ParentID); err != nil {
		return nil, err
	}
	
	return allFiles, nil
}

// SearchFiles searches for files by name across the entire account
func (s *StoreClient) SearchFiles(params *store.SearchFilesParams) (*store.ListAllFilesData, error) {
	if params.Query == "" {
		return nil, errors.New("search query cannot be empty")
	}
	
	ctx := Ctx{Ctx: params.Ctx}
	
	if params.PageSize == 0 {
		params.PageSize = 100
	}
	
	// Use PikPak's list files and filter locally
	res, err := s.client.ListFiles(&ListFilesParams{
		Ctx:   ctx,
		Limit: params.PageSize,
		Filters: map[string]map[string]any{
			"trashed": {"eq": false},
		},
		PageToken: params.PageToken,
	})
	if err != nil {
		return nil, err
	}
	
	// Filter results by query string (case-insensitive)
	files := make([]store.FileItem, 0)
	query := strings.ToLower(params.Query)
	
	for _, f := range res.Data.Files {
		if strings.Contains(strings.ToLower(f.Name), query) {
			fileType := store.FileTypeFile
			if f.Kind == FileKindFolder {
				fileType = store.FileTypeFolder
			}
			
			createdAt, _ := time.Parse(time.RFC3339, f.CreatedTime)
			modifiedAt, _ := time.Parse(time.RFC3339, f.ModifiedTime)
			
			fileItem := store.FileItem{
				ID:         f.Id,
				Name:       f.Name,
				Size:       toSize(f.Size),
				Type:       fileType,
				MimeType:   f.MimeType,
				CreatedAt:  createdAt,
				ModifiedAt: modifiedAt,
				ParentID:   f.ParentId,
				Hash:       f.Hash,
			}
			
			if len(f.Medias) > 0 && f.Medias[0].Link.URL != "" {
				fileItem.DownloadLink = f.Medias[0].Link.URL
			}
			
			files = append(files, fileItem)
		}
	}
	
	data := &store.ListAllFilesData{
		Files:         files,
		NextPageToken: res.Data.NextPageToken,
		HasMore:       res.Data.NextPageToken != "",
	}
	
	return data, nil
}

// GetFileDetails gets detailed information about a specific file
func (s *StoreClient) GetFileDetails(params *store.GetFileDetailsParams) (*store.FileItem, error) {
	ctx := Ctx{Ctx: params.Ctx}
	
	res, err := s.client.GetFile(&GetFileParams{
		Ctx:    ctx,
		FileId: params.FileID,
	})
	if err != nil {
		return nil, err
	}
	
	fileType := store.FileTypeFile
	if res.Data.Kind == FileKindFolder {
		fileType = store.FileTypeFolder
	}
	
	createdAt, _ := time.Parse(time.RFC3339, res.Data.CreatedTime)
	modifiedAt, _ := time.Parse(time.RFC3339, res.Data.ModifiedTime)
	
	fileItem := &store.FileItem{
		ID:         res.Data.Id,
		Name:       res.Data.Name,
		Size:       toSize(res.Data.Size),
		Type:       fileType,
		MimeType:   res.Data.MimeType,
		CreatedAt:  createdAt,
		ModifiedAt: modifiedAt,
		ParentID:   res.Data.ParentId,
		Hash:       res.Data.Hash,
	}
	
	if len(res.Data.Medias) > 0 && res.Data.Medias[0].Link.URL != "" {
		fileItem.DownloadLink = res.Data.Medias[0].Link.URL
	}
	
	return fileItem, nil
}

// ListAllFilesFlat lists all files in a flat structure with full paths
func (s *StoreClient) ListAllFilesFlat(params *store.ListAllFilesFlatParams) ([]store.FileItem, error) {
	ctx := Ctx{Ctx: params.Ctx}
	allFiles := []store.FileItem{}
	
	var traverse func(folderID string, currentPath string) error
	traverse = func(folderID string, currentPath string) error {
		listParams := &ListFilesParams{
			Ctx:      ctx,
			ParentId: folderID,
			Filters: map[string]map[string]any{
				"trashed": {"eq": false},
			},
		}
		
		res, err := s.client.ListFiles(listParams)
		if err != nil {
			return err
		}
		
		for _, f := range res.Data.Files {
			fileType := store.FileTypeFile
			if f.Kind == FileKindFolder {
				fileType = store.FileTypeFolder
			}
			
			createdAt, _ := time.Parse(time.RFC3339, f.CreatedTime)
			modifiedAt, _ := time.Parse(time.RFC3339, f.ModifiedTime)
			
			filePath := path.Join(currentPath, f.Name)
			
			fileItem := store.FileItem{
				ID:         f.Id,
				Name:       f.Name,
				Size:       toSize(f.Size),
				Type:       fileType,
				MimeType:   f.MimeType,
				CreatedAt:  createdAt,
				ModifiedAt: modifiedAt,
				ParentID:   f.ParentId,
				Hash:       f.Hash,
				Path:       filePath,
			}
			
			if len(f.Medias) > 0 && f.Medias[0].Link.URL != "" {
				fileItem.DownloadLink = f.Medias[0].Link.URL
			}
			
			if f.Kind == FileKindFolder {
				// Traverse into folder
				if err := traverse(f.Id, filePath); err != nil {
					return err
				}
			} else {
				// Only add files, not folders
				allFiles = append(allFiles, fileItem)
			}
		}
		
		return nil
	}
	
	if err := traverse(params.ParentID, "/"); err != nil {
		return nil, err
	}
	
	return allFiles, nil
}

// GetFolderByName finds a folder by name in the root or specified parent
func (s *StoreClient) GetFolderByName(params *store.GetFolderByNameParams) (*store.FileItem, error) {
	ctx := Ctx{Ctx: params.Ctx}
	
	listParams := &ListFilesParams{
		Ctx:      ctx,
		ParentId: params.ParentID,
		Filters: map[string]map[string]any{
			"trashed": {"eq": false},
			"kind":    {"eq": FileKindFolder},
		},
	}
	
	res, err := s.client.ListFiles(listParams)
	if err != nil {
		return nil, err
	}
	
	for _, f := range res.Data.Files {
		if f.Name == params.FolderName {
			createdAt, _ := time.Parse(time.RFC3339, f.CreatedTime)
			modifiedAt, _ := time.Parse(time.RFC3339, f.ModifiedTime)
			
			return &store.FileItem{
				ID:         f.Id,
				Name:       f.Name,
				Size:       toSize(f.Size),
				Type:       store.FileTypeFolder,
				MimeType:   f.MimeType,
				CreatedAt:  createdAt,
				ModifiedAt: modifiedAt,
				ParentID:   f.ParentId,
			}, nil
		}
	}
	
	return nil, errors.New("folder '" + params.FolderName + "' not found")
}

// ListVideoFiles lists only video files (helper method)
func (s *StoreClient) ListVideoFiles(params *store.ListAllFilesParams) (*store.ListAllFilesData, error) {
	data, err := s.ListAllFiles(params)
	if err != nil {
		return nil, err
	}
	
	// Filter for video files
	videoFiles := []store.FileItem{}
	videoMimeTypes := []string{
		"video/mp4",
		"video/x-matroska",
		"video/avi",
		"video/quicktime",
		"video/x-msvideo",
		"video/webm",
		"video/mpeg",
	}
	
	for _, file := range data.Files {
		if file.Type == store.FileTypeFile {
			// Check by MIME type
			for _, mimeType := range videoMimeTypes {
				if file.MimeType == mimeType {
					videoFiles = append(videoFiles, file)
					break
				}
			}
		}
	}
	
	return &store.ListAllFilesData{
		Files:         videoFiles,
		NextPageToken: data.NextPageToken,
		HasMore:       data.HasMore,
	}, nil
}