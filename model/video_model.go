package model

type VideoManifestRequest struct {
	VideoID  string `json:"video_id"`
	Playlist string `json:"playlist"`
}

type VideoKeyRequest struct {
	VideoID  string `json:"video_id"`
	Playlist string `json:"playlist"`
	KeyName  string `json:"key_name"`
}
