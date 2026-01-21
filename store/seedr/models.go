package seedr

// -------- Account --------

type AccountResponse struct {
	Email     string `json:"email"`
	SpaceMax  int64  `json:"space_max"`
	SpaceUsed int64  `json:"space_used"`
	Username  string `json:"username"`
}

// -------- Folders list --------

type FoldersResponse struct {
	Folders []SeedrFolder `json:"folders"`
	Files   []SeedrFile   `json:"files"`
}

type SeedrFolder struct {
	Id         int    `json:"id"`
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	LastUpdate string `json:"last_update"`
}

type SeedrFile struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// -------- Folder contents --------

type FolderResponse struct {
	Folders []SeedrFolder `json:"folders"`
	Files   []SeedrFile   `json:"files"`
}

// -------- File info / stream URL --------

type FileResponse struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
	URL  string `json:"url"`
}