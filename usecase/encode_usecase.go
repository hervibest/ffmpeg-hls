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
	OutputDir string
	S3Prefix  string
	APIServer string
	VideoID   string
	InputPath string
}

func EncodeAndUpload(ctx context.Context, req EncodeRequest, minioClient *util.MinioClient) error {
	req.InputPath = filepath.Join("tmp", fmt.Sprintf("%s.mp4", req.VideoID))
	req.OutputDir = filepath.Join("tmp/output", req.VideoID)
	req.S3Prefix = fmt.Sprintf("courses/%s", req.VideoID)

	if err := os.MkdirAll(req.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	for label, res := range resolutions {
		if err := encodeVariant(ctx, req, label, res.Width, res.Height, res.Bitrate); err != nil {
			return err
		}
	}

	if err := generateMasterPlaylist(req.OutputDir); err != nil {
		return fmt.Errorf("failed to generate master playlist: %w", err)
	}

	err := filepath.WalkDir(req.OutputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		relPath := filepath.Base(path)
		key := fmt.Sprintf("%s/%s", req.S3Prefix, relPath)
		if strings.HasSuffix(relPath, ".key") {
			key = fmt.Sprintf("%s/secrets/%s", req.S3Prefix, relPath)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return minioClient.UploadToS3(ctx, minioClient.GetBucketName(), key, data)
	})
	if err != nil {
		return err
	}

	return util.DeleteDir(req.OutputDir)
}

func encodeVariant(ctx context.Context, req EncodeRequest, label string, width, height int, bitrate string) error {
	playlist := filepath.Join(req.OutputDir, fmt.Sprintf("%s.m3u8", label))
	segmentPattern := filepath.Join(req.OutputDir, fmt.Sprintf("%s_%%03d.ts", label))

	keyBin := make([]byte, 16)
	if _, err := rand.Read(keyBin); err != nil {
		return fmt.Errorf("failed to generate key: %w", err)
	}
	keyPath := filepath.Join(req.OutputDir, fmt.Sprintf("enc_%s.key", label))
	keyUriPlaceholder := fmt.Sprintf("enc_%s.key", label)
	keyInfo := filepath.Join(req.OutputDir, fmt.Sprintf("keyinfo_%s.txt", label))

	if err := os.WriteFile(keyPath, keyBin, 0644); err != nil {
		return err
	}

	iv := make([]byte, 16)
	rand.Read(iv)
	ivHex := hex.EncodeToString(iv)
	keyInfoContent := fmt.Sprintf("%s\n%s\n%s", keyUriPlaceholder, keyPath, ivHex)
	if err := os.WriteFile(keyInfo, []byte(keyInfoContent), 0644); err != nil {
		return fmt.Errorf("failed to write keyinfo: %w", err)
	}

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", req.InputPath,
		"-vf", fmt.Sprintf("scale=w=%d:h=%d", width, height),
		"-c:a", "aac",
		"-b:v", bitrate,
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

	finalKeyUri := fmt.Sprintf("%s/videos/%s/keys/enc_%s.key", strings.TrimSuffix(req.APIServer, "/"), req.VideoID, label)
	m3u8Path := filepath.Join(req.OutputDir, fmt.Sprintf("%s.m3u8", label))
	data, err := os.ReadFile(m3u8Path)
	if err != nil {
		return fmt.Errorf("failed to read m3u8: %w", err)
	}
	updated := strings.ReplaceAll(string(data), keyUriPlaceholder, finalKeyUri)
	return os.WriteFile(m3u8Path, []byte(updated), 0644)
}

func generateMasterPlaylist(outputDir string) error {
	var builder strings.Builder
	builder.WriteString("#EXTM3U\n")

	for label, res := range resolutions {
		bandwidth := map[string]string{
			"360p":  "800000",
			"480p":  "1400000",
			"720p":  "2800000",
			"1080p": "5000000",
		}[label]
		builder.WriteString(fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%s,RESOLUTION=%dx%d\n", bandwidth, res.Width, res.Height))
		builder.WriteString(fmt.Sprintf("%s.m3u8\n", label))
	}

	masterPath := filepath.Join(outputDir, "master.m3u8")
	return os.WriteFile(masterPath, []byte(builder.String()), 0644)
}
