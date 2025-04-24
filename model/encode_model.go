package model

type EncodeRequest struct {
	OutputDir string `json:"output_dir"`
	S3Prefix  string `json:"s3_prefix"`
	APIServer string `json:"api_server"`
	VideoID   string `json:"video_id"`
	InputPath string `json:"input_path"`
	Playlist  string `json:"playlist"`
}
