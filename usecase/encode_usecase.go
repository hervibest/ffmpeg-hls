package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"ffmpeg-hls/util"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var resolutions = map[string]struct {
	Bitrate string
	Width   int
	Height  int
}{
	"360p":  {"500k", 480, 360},
	"480p":  {"1000k", 858, 480},
	"720p":  {"2000k", 1280, 720},
	"1080p": {"4000k", 1920, 1080},
}

type EncodeRequest struct {
	InputPath string // e.g. "/tmp/input.mp4"
	OutputDir string // e.g. "/tmp/output/video456/"
	S3Prefix  string // e.g. "courses/123/video456"
	Bucket    string // e.g. "ffmpeg"

	APIServer string // e.g. "https://api.example.com"
	VideoID   string // the same ID your Go handler uses
}

func EncodeAndUpload(ctx context.Context, req EncodeRequest) error {
	if err := os.MkdirAll(req.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	for label, res := range resolutions {
		playlist := filepath.Join(req.OutputDir, fmt.Sprintf("%s.m3u8", label))
		segmentPattern := filepath.Join(req.OutputDir, fmt.Sprintf("%s_%%03d.ts", label))

		keyBin := make([]byte, 16)
		_, err := rand.Read(keyBin)
		if err != nil {
			return fmt.Errorf("failed to generate key: %w", err)
		}
		keyPath := filepath.Join(req.OutputDir, fmt.Sprintf("enc_%s.key", label))
		// cleanPrefix := strings.TrimSuffix(req.S3Prefix, "/")
		keyUriPlaceholder := fmt.Sprintf("enc_%s.key", label) // SIMPLE placeholder

		keyInfo := filepath.Join(req.OutputDir, fmt.Sprintf("keyinfo_%s.txt", label))

		err = os.WriteFile(keyPath, keyBin, 0644)
		if err != nil {
			return err
		}

		iv := make([]byte, 16)
		rand.Read(iv)
		ivHex := hex.EncodeToString(iv)
		ivLine := ivHex
		keyInfoContent := strings.Join([]string{
			keyUriPlaceholder,
			keyPath,
			ivLine,
		}, "\n")

		if err := os.WriteFile(keyInfo, []byte(keyInfoContent), 0644); err != nil {
			return fmt.Errorf("failed to write keyinfo: %w", err)
		}

		// ✅ Debug setelah file keyinfo ditulis dengan benar
		if data, err := os.ReadFile(keyInfo); err == nil {
			fmt.Printf("[DEBUG] keyinfo content for %s:\n%s\n", label, string(data))
		}

		cmd := exec.CommandContext(ctx, "ffmpeg",
			"-i", req.InputPath,
			"-vf", fmt.Sprintf("scale=w=%d:h=%d", res.Width, res.Height),
			"-c:a", "aac",
			"-b:v", res.Bitrate,
			"-hls_time", "4",
			"-hls_playlist_type", "vod",
			"-hls_segment_filename", segmentPattern,
			"-hls_key_info_file", keyInfo,
			playlist,
		)

		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("ffmpeg failed for %s: %w", label, err)
		}

		// ✅ Replace key URI setelah file m3u8 berhasil dibuat
		m3u8Path := filepath.Join(req.OutputDir, fmt.Sprintf("%s.m3u8", label))
		// Build the key URL for our Go REST endpoint:
		//   GET /videos/:videoID/keys/enc_<label>.key
		finalKeyUri := fmt.Sprintf(
			"%s/videos/%s/keys/enc_%s.key",         // format: baseURL + path
			strings.TrimSuffix(req.APIServer, "/"), // e.g. "https://api.example.com"
			req.VideoID,                            // the :videoID path parameter
			label,                                  // resolution label, e.g. "360p"
		)

		data, err := os.ReadFile(m3u8Path)
		if err != nil {
			return fmt.Errorf("failed to read m3u8: %w", err)
		}
		updated := strings.ReplaceAll(string(data), keyUriPlaceholder, finalKeyUri)
		if err := os.WriteFile(m3u8Path, []byte(updated), 0644); err != nil {
			return fmt.Errorf("failed to write final m3u8: %w", err)
		}
	}

	// Upload ke S3
	err := filepath.WalkDir(req.OutputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		relPath := filepath.Base(path)
		key := fmt.Sprintf("%s/%s", req.S3Prefix, relPath)

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return util.UploadToS3(ctx, req.Bucket, key, data)
	})
	if err != nil {
		return err
	}

	// Cleanup
	return util.DeleteDir(req.OutputDir)
}
