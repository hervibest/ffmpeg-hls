package usecase

import (
	"context"
	"ffmpeg-hls/model"
	"ffmpeg-hls/repository"
	"ffmpeg-hls/util"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/minio/minio-go/v7"
)

type VideoUseCase interface {
	VideoManifest(ctx context.Context, req *model.VideoManifestRequest) ([]string, error)
	VideoKey(ctx context.Context, req *model.VideoKeyRequest) ([]byte, error)
}

type videoUseCase struct {
	minio           *util.Minio
	videoRepository repository.VideoRepository
}

func NewVideoUseCase(minio *util.Minio, videoRepository repository.VideoRepository) VideoUseCase {
	return &videoUseCase{
		minio:           minio,
		videoRepository: videoRepository,
	}
}

func (u *videoUseCase) VideoManifest(ctx context.Context, req *model.VideoManifestRequest) ([]string, error) {
	video, err := u.videoRepository.GetByID(ctx, req.VideoID)
	if err != nil {
		log.Print(fmt.Sprint("[CLIENT][[USECASE][GetById] error : %w", err))
		return nil, fiber.NewError(http.StatusNotFound, "Requested video not found")
	}

	decodedDir, err := url.PathUnescape(video.Dir)
	if err != nil {
		log.Print(fmt.Sprint("[INTERNAL][[USECASE][DecodeUrl] error : %w", err))
		return nil, fiber.NewError(http.StatusInternalServerError, "Something wrong please try again later.")
	}

	key := fmt.Sprintf("%s/%s", decodedDir, req.Playlist)
	obj, err := u.minio.GetObject(ctx, u.minio.GetBucketName(), key, minio.GetObjectOptions{})
	if err != nil {
		log.Print(fmt.Sprint("[INTERNAL][[USECASE][GetObject] error : %w", err))
		return nil, fiber.NewError(http.StatusInternalServerError, "Something wrong please try again later.")
	}

	log.Println("ini video dir", video.Dir)
	log.Println("ini key", key)

	log.Println("ini playlist", req.Playlist)

	defer obj.Close()

	raw, err := io.ReadAll(obj)
	if err != nil {
		log.Print(fmt.Sprint("[INTERNAL][[USECASE][IoReadAll] error : %w", err))
		return nil, fiber.NewError(http.StatusInternalServerError, "Something wrong please try again later.")
	}

	lines := strings.Split(string(raw), "\n")
	for i, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)

		if strings.HasPrefix(trimmed, "#EXT-X-KEY") {
			log.Println("ini adalah trimmed ", trimmed)
			lines[i] = trimmed
			continue
		}

		if strings.HasSuffix(trimmed, ".ts") {
			seg := filepath.Base(trimmed)
			decodedDir, err := url.PathUnescape(video.Dir)
			url, err := u.minio.PresignedGetObject(ctx, u.minio.GetBucketName(), fmt.Sprintf("%s/%s", decodedDir, seg), time.Hour, nil)
			if err != nil {
				log.Print(fmt.Sprint("[INTERNAL][USECASE][PresignedGetObject] error : %w", err))
				return nil, fiber.NewError(http.StatusInternalServerError, "Something wrong please try again later.")
			}
			lines[i] = url.String()
		}
	}

	return lines, nil
}

func (u *videoUseCase) VideoKey(ctx context.Context, req *model.VideoKeyRequest) ([]byte, error) {
	video, err := u.videoRepository.GetByID(ctx, req.VideoID)
	if err != nil {
		log.Print(fmt.Sprint("[CLIENT][[USECASE][GetById] error : %w", err))
		return nil, fiber.NewError(http.StatusNotFound, "Requested video not found")
	}

	decodedDir, err := url.PathUnescape(video.Dir)
	if err != nil {
		log.Print(fmt.Sprint("[INTERNAL][[USECASE][DecodeUrl] error : %w", err))
		return nil, fiber.NewError(http.StatusInternalServerError, "Something wrong please try again later.")
	}

	keyPath := fmt.Sprintf("%s/secrets/%s", decodedDir, req.KeyName)
	log.Println("KeyPath:", keyPath)

	obj, err := u.minio.GetObject(ctx, u.minio.GetBucketName(), keyPath, minio.GetObjectOptions{})
	if err != nil {
		log.Print(fmt.Sprint("[INTERNAL][[USECASE][GetObject] error : %w", err))
		return nil, fiber.NewError(http.StatusInternalServerError, "Something wrong please try again later.")
	}

	data, err := io.ReadAll(obj)
	if err != nil {
		log.Print("Failed to read all from object:", err)
		log.Print(fmt.Sprint("[INTERNAL][[USECASE][IoReadAll] error : %w", err))
		return nil, fiber.NewError(http.StatusInternalServerError, "Something wrong please try again later.")

	}

	return data, nil
}
