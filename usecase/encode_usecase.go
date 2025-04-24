package usecase

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"ffmpeg-hls/model"
	"ffmpeg-hls/util"
	errorcode "ffmpeg-hls/util/error"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type EncodeUseCase interface {
	EncodeAndUpload(ctx context.Context, req *model.EncodeRequest) error
}

type encodeUseCase struct {
	minio *util.Minio
}

func NewEncodeUseCase(minio *util.Minio) EncodeUseCase {
	return &encodeUseCase{minio: minio}
}

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

func (u *encodeUseCase) EncodeAndUpload(ctx context.Context, req *model.EncodeRequest) error {
	req.InputPath = filepath.Join("tmp", fmt.Sprintf("%s.mp4", req.VideoID))
	req.OutputDir = filepath.Join("tmp/output", req.VideoID)
	req.S3Prefix = fmt.Sprintf("courses/%s", req.VideoID)

	if err := os.MkdirAll(req.OutputDir, 0755); err != nil {
		log.Printf("[USECASE][MkdirAll] %v", err)
		return fiber.NewError(http.StatusInternalServerError, errorcode.INTERNAL_SERVER_ERROR)
	}

	for label, res := range resolutions {
		if err := u.encodeVariant(ctx, req, label, res.Width, res.Height, res.Bitrate); err != nil {
			log.Printf("[USECASE][EncodeVariant %s] %v", label, err)
			return fiber.NewError(http.StatusInternalServerError, errorcode.INTERNAL_SERVER_ERROR)
		}
	}

	if err := generateMasterPlaylist(req.OutputDir); err != nil {
		log.Printf("[USECASE][GenerateMasterPlaylist] %v", err)
		return fiber.NewError(http.StatusInternalServerError, errorcode.INTERNAL_SERVER_ERROR)
	}

	if err := u.uploadDirToS3(ctx, req); err != nil {
		log.Printf("[USECASE][UploadDir] %v", err)
		return fiber.NewError(http.StatusInternalServerError, errorcode.INTERNAL_SERVER_ERROR)
	}

	return util.DeleteDir(req.OutputDir)
}

func (u *encodeUseCase) encodeVariant(ctx context.Context, req *model.EncodeRequest, label string, width, height int, bitrate string) error {
	playlist := filepath.Join(req.OutputDir, fmt.Sprintf("%s.m3u8", label))
	segmentPattern := filepath.Join(req.OutputDir, fmt.Sprintf("%s_%%03d.ts", label))

	keyBin := make([]byte, 16)
	if _, err := rand.Read(keyBin); err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	keyPath := filepath.Join(req.OutputDir, fmt.Sprintf("enc_%s.key", label))
	if err := os.WriteFile(keyPath, keyBin, 0644); err != nil {
		return fmt.Errorf("write key file: %w", err)
	}

	iv := make([]byte, 16)
	rand.Read(iv)
	ivHex := hex.EncodeToString(iv)

	keyUriPlaceholder := fmt.Sprintf("__REPLACE_ME_URI_%s__", label)
	keyInfoPath := filepath.Join(req.OutputDir, fmt.Sprintf("keyinfo_%s.txt", label))
	keyInfoContent := fmt.Sprintf("%s\n%s\n%s", keyUriPlaceholder, keyPath, ivHex)

	if err := os.WriteFile(keyInfoPath, []byte(keyInfoContent), 0644); err != nil {
		return fmt.Errorf("write keyinfo: %w", err)
	}
	defer os.Remove(keyInfoPath) // optional cleanup

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", req.InputPath,
		"-vf", fmt.Sprintf("scale=w=%d:h=%d", width, height),
		"-c:a", "aac",
		"-b:v", bitrate,
		"-hls_time", "4",
		"-hls_playlist_type", "vod",
		"-hls_segment_filename", segmentPattern,
		"-hls_key_info_file", keyInfoPath,
		playlist,
	)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg run failed: %w", err)
	}

	return replaceKeyUriInM3U8(req.OutputDir, label, req.APIServer, req.VideoID, keyUriPlaceholder)
}

func replaceKeyUriInM3U8(outputDir, label, apiServer, videoID, placeholder string) error {
	m3u8Path := filepath.Join(outputDir, fmt.Sprintf("%s.m3u8", label))
	data, err := os.ReadFile(m3u8Path)
	if err != nil {
		return fmt.Errorf("read m3u8 file: %w", err)
	}

	finalKeyUri := fmt.Sprintf("%s/videos/%s/keys/enc_%s.key", strings.TrimSuffix(apiServer, "/"), videoID, label)
	log.Println("[DEBUG] Final Key URI:", finalKeyUri)
	updated := bytes.ReplaceAll(data, []byte(placeholder), []byte(finalKeyUri))

	return os.WriteFile(m3u8Path, updated, 0644)
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

func (u *encodeUseCase) uploadDirToS3(ctx context.Context, req *model.EncodeRequest) error {
	return filepath.WalkDir(req.OutputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(req.OutputDir, path)
		if err != nil {
			return fmt.Errorf("rel path: %w", err)
		}

		key := fmt.Sprintf("%s/%s", req.S3Prefix, relPath)
		if strings.HasSuffix(relPath, ".key") {
			key = fmt.Sprintf("%s/secrets/%s", req.S3Prefix, filepath.Base(relPath))
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		return u.minio.UploadToS3(ctx, u.minio.GetBucketName(), key, data)
	})
}
