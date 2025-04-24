// main.go
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// --- Video Metadata Service ---
type Video struct {
	Dir string // e.g. "courses/123/video456"
}

type VideoService interface {
	GetByID(ctx context.Context, id string) (*Video, error)
}

type dummyVideoService struct{}

func (d *dummyVideoService) GetByID(ctx context.Context, id string) (*Video, error) {
	return &Video{Dir: fmt.Sprintf("courses/%s", id)}, nil
}

// --- HLS Playlist Handler ---
func videoManifestHandler(mc *minio.Client, svc VideoService) fiber.Handler {
	bucket := os.Getenv("MINIO_TICKETS_BUCKET")

	return func(c *fiber.Ctx) error {

		log.Print("accessed")
		ctx := context.Background()
		vid := c.Params("videoID")
		playlist := c.Params("playlist") // e.g. "360p.m3u8" or "master.m3u8"

		video, err := svc.GetByID(ctx, vid)
		if err != nil {
			return c.Status(404).SendString("video not found")
		}

		decodedDir, err := url.PathUnescape(video.Dir)
		if err != nil {
			return c.Status(500).SendString("failed to decode video dir")
		}

		key := fmt.Sprintf("%s/%s", decodedDir, playlist)
		obj, err := mc.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}

		log.Println("ini bucket ", bucket)
		log.Println("ini video dir", video.Dir)
		log.Println("ini key", key)

		log.Println("ini playlist", playlist)

		defer obj.Close()

		raw, err := io.ReadAll(obj)
		if err != nil {
			log.Println("ini err", err)
			return c.Status(500).SendString(err.Error())
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
				url, err := mc.PresignedGetObject(ctx, bucket, fmt.Sprintf("%s/%s", decodedDir, seg), time.Hour, nil)
				if err != nil {
					return c.Status(500).SendString(err.Error())
				}
				lines[i] = url.String()
			}
		}

		c.Type("application/vnd.apple.mpegurl", "utf-8")
		return c.SendString(strings.Join(lines, "\n"))
	}
}

func videoKeyHandler(mc *minio.Client, svc VideoService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		bucket := os.Getenv("MINIO_TICKETS_BUCKET")
		log.Print("accessed")

		ctx := context.Background()
		vid := c.Params("videoID")
		keyName := c.Params("key")

		video, err := svc.GetByID(ctx, vid)
		if err != nil {
			log.Print("Video not found:", err)
			return c.Status(404).SendString("video not found")
		}

		decodedDir, err := url.PathUnescape(video.Dir)
		if err != nil {
			return c.Status(500).SendString("failed to decode video dir")
		}

		keyPath := fmt.Sprintf("%s/secrets/%s", decodedDir, keyName)
		log.Println("Bucket:", bucket)
		log.Println("KeyPath:", keyPath)

		obj, err := mc.GetObject(ctx, bucket, keyPath, minio.GetObjectOptions{})
		if err != nil {
			log.Print("GetObject failed:", err)
			return c.Status(404).SendString("key not found")
		}

		// Read content into memory
		data, err := io.ReadAll(obj)
		if err != nil {
			log.Print("Failed to read all from object:", err)
			return c.Status(500).SendString("failed to read key content")
		}

		log.Printf("Read %d bytes", len(data))
		c.Type("application/octet-stream")
		c.Response().Header.Set("Content-Length", fmt.Sprintf("%d", len(data)))
		return c.SendStream(bytes.NewReader(data)) // or c.Send(data) if prefered
	}
}

// // --- Helper to extract key filename ---
// func extractKeyFileName(line string) string {
// 	uriStart := strings.Index(line, "URI=\"") + len("URI=\"")
// 	uriEnd := strings.Index(line[uriStart:], "\"")
// 	return line[uriStart : uriStart+uriEnd]
// }

func main() {
	if err := godotenv.Load(".env"); err != nil {
		panic(err)
	}

	endpoint := os.Getenv("MINIO_HOST") + ":" + os.Getenv("MINIO_PORT")
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(os.Getenv("MINIO_ROOT_USER"), os.Getenv("MINIO_ROOT_PASSWORD"), ""),
		Secure: os.Getenv("MINIO_USE_SSL") == "true",
	})
	if err != nil {
		log.Fatalf("failed to initialize MinIO client: %v", err)
	}

	svc := &dummyVideoService{}
	app := fiber.New()

	// ✅ Tambahkan middleware CORS di sini
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*", // atau spesifik: "http://127.0.0.1:5500"
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	app.Get("/videos/:videoID/playlists/:playlist", videoManifestHandler(mc, svc))
	app.Get("/videos/:videoID/keys/:key", videoKeyHandler(mc, svc))

	log.Println("listening on :5000")
	baseUrl := os.Getenv("BASE_IP_URL")
	log.Fatal(app.Listen(fmt.Sprintf("%s:5000", baseUrl)))
}
