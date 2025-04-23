// main.go
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// --- stub of your video lookup service ---
type Video struct {
	Dir string // e.g. "courses/123/video456"
}
type VideoService interface {
	GetByID(ctx context.Context, id string) (*Video, error)
}
type dummyVideoService struct{}

func (d *dummyVideoService) GetByID(ctx context.Context, id string) (*Video, error) {
	return &Video{Dir: fmt.Sprintf("courses/123/video%s", id)}, nil
}

// --- playlist handler ---
func videoManifestHandler(mc *minio.Client, svc VideoService) fiber.Handler {
	bucket := os.Getenv("MINIO_TICKETS_BUCKET")

	return func(c *fiber.Ctx) error {
		ctx := context.Background()
		vid := c.Params("videoID")
		playlist := c.Params("playlist") // e.g. "360p.m3u8"

		// 1) lookup video directory
		video, err := svc.GetByID(ctx, vid)
		if err != nil {
			return c.Status(404).SendString("video not found")
		}

		log.Print(filepath.Join(bucket, video.Dir, playlist))

		// 2) fetch the raw .m3u8 from MinIO
		obj, err := mc.GetObject(ctx, bucket, filepath.Join(video.Dir, playlist), minio.GetObjectOptions{})
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		log.Print("ACCESED 2")

		defer obj.Close()

		raw, err := io.ReadAll(obj)
		if err != nil {
			log.Print("error")

			return c.Status(500).SendString(err.Error())
		}

		log.Print("ACCESED 3")

		// 3) rewrite setiap baris
		lines := strings.Split(string(raw), "\n")
		for i, rawLine := range lines {
			// buang spasi + CR untuk pemeriksaan
			trimmed := strings.TrimSpace(rawLine)

			// 3a) rewrite kunci
			if strings.HasPrefix(trimmed, "#EXT-X-KEY") {
				// …(kode Anda yang sudah diperbaiki sebelumnya)…
				continue
			}

			// 3b) rewrite segmen .ts
			if strings.HasSuffix(trimmed, ".ts") {
				seg := filepath.Base(trimmed) // "360p_000.ts"
				url, err := mc.PresignedGetObject(
					ctx, bucket, filepath.Join(video.Dir, seg),
					time.Hour, nil,
				)
				if err != nil {
					return c.Status(500).SendString(err.Error())
				}
				// pertahankan kemungkinan '\r'
				if strings.HasSuffix(rawLine, "\r") {
					lines[i] = url.String() + "\r"
				} else {
					lines[i] = url.String()
				}
			}
		}

		// 4) kirim kembali sebagai HLS manifest
		c.Type("application/vnd.apple.mpegurl", "utf-8")
		return c.SendString(strings.Join(lines, "\n"))
	}
}

// --- key proxy handler ---
func videoKeyHandler(mc *minio.Client, svc VideoService) fiber.Handler {
	log.Print("ACCESSED KEY REQUEST")
	bucket := os.Getenv("MINIO_TICKETS_BUCKET")

	return func(c *fiber.Ctx) error {
		log.Print("ACCESSED KEY REQUEST")
		ctx := context.Background()
		vid := c.Params("videoID")
		keyName := c.Params("key") // e.g. "enc_360p.key"

		video, err := svc.GetByID(ctx, vid)
		if err != nil {
			return c.Status(404).SendString("video not found")
		}

		obj, err := mc.GetObject(ctx, bucket, filepath.Join(video.Dir, "secrets", keyName), minio.GetObjectOptions{})
		if err != nil {
			return c.Status(404).SendString("key not found")
		}
		defer obj.Close()

		c.Type("application/octet-stream")
		return c.SendStream(obj)
	}
}

func TestHandler(c *fiber.Ctx) error {
	log.Print("ACCESSED KEY REQUEST")
	return nil
}

func main() {
	// Inisialisasi MinIO client

	if err := godotenv.Load(".env"); err != nil {
		panic(err)
	}

	minioHost := os.Getenv("MINIO_HOST")
	minioPort := os.Getenv("MINIO_PORT")
	minioRootUser := os.Getenv("MINIO_ROOT_USER")
	minioRootPassword := os.Getenv("MINIO_ROOT_PASSWORD")
	endpoint := minioHost + ":" + minioPort

	useSSL := os.Getenv("MINIO_USE_SSL") == "true"

	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioRootUser, minioRootPassword, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Fatalf("failed to initialize MinIO client: %v", err)
	}

	svc := &dummyVideoService{}
	app := fiber.New()

	app.Get("/videos/:videoID/playlists/:playlist", videoManifestHandler(mc, svc))
	app.Get("/videos/:videoID/keys/:key", videoKeyHandler(mc, svc))

	log.Println("listening on :5000")
	log.Fatal(app.Listen(":5000"))
}
