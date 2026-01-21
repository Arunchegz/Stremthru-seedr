package seedr

import "time"

// API Response Types

type APIError struct {
	Error  string `json:"error"`
	Result bool   `json:"result"`
}

type SettingsResponse struct {
	Result   bool `json:"result"`
	Settings struct {
		Account struct {
			Email          string `json:"email"`
			Username       string `json:"username"`
			IsPremium      bool   `json:"is_premium"`
			BandwidthUsed  int64  `json:"bandwidth_used"`
			BandwidthMax   int64  `json:"bandwidth_max"`
			SpaceUsed      int64  `json:"space_used"`
			SpaceMax       int64  `json:"space_max"`
			PremiumUntil   string `json:"premium_until"`
		} `json:"account"`
	} `json:"settings"`
}

type AddMagnetRequest struct {
	Magnet string `json:"magnet"`
}

type AddMagnetResponse struct {
	Result         bool   `json:"result"`
	UserTorrent    int    `json:"user_torrent_id"`
	TorrentHash    string `json:"hash"`
	Title          string `json:"title"`
}

type FolderResponse struct {
	Result  bool     `json:"result"`
	Folders []Folder `json:"folders"`
	Files   []File   `json:"files"`
	Name    string   `json:"name"`
	ID      int      `json:"id"`
}

type Folder struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	FullName    string    `json:"fullname"`
	Size        int64     `json:"size"`
	LastUpdate  time.Time `json:"last_update"`
}

type File struct {
	FolderFileID int       `json:"folder_file_id"`
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	URL          string    `json:"url"`
	PlayVideo    string    `json:"play_video"`
	LastUpdate   time.Time `json:"last_update"`
}

type DeleteFolderResponse struct {
	Result bool `json:"result"`
}

type DeleteFileResponse struct {
	Result bool `json:"result"`
}

// Helper Types

type TorrentStatus struct {
	ID          int
	Hash        string
	Name        string
	Size        int64
	Progress    float64 // 0-100 downloading, 101 = complete
	Folders     []Folder
	Files       []File
}