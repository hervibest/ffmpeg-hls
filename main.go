// main.go
package main

import (
	"context"
	"ffmpeg-hls/handler"
	"ffmpeg-hls/repository"
	"ffmpeg-hls/usecase"
	"ffmpeg-hls/util"
	"ffmpeg-hls/worker"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(".env"); err != nil {
		panic(err)
	}

	minio := util.InitMinio()

	videoRepo := repository.NewVideoRepository()

	encodeUC := usecase.NewEncodeUseCase(minio)
	videoUC := usecase.NewVideoUseCase(minio, videoRepo)
	encodeWorker := worker.NewEncodeWorker(1, encodeUC)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		encodeWorker.Run(ctx)
	}()

	encodeHandler := handler.NewEncodeHandler(encodeUC, encodeWorker)
	videoHandler := handler.NewVideoHandler(videoUC)

	app := fiber.New()

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	app.Get("/videos/:videoID/playlists/:playlist", videoHandler.VideoManifest)
	app.Get("/videos/:videoID/keys/:key", videoHandler.VideoKey)

	app.Post("/video/upload", encodeHandler.UploadVideo)

	interuptSignal := make(chan os.Signal, 1)
	signal.Notify(interuptSignal, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-interuptSignal
		log.Printf("[MAIN] received shutdown signal: %v", sig)

		cancel()
		time.Sleep(2 * time.Second)

		if err := app.Shutdown(); err != nil {
			log.Printf("[MAIN] graceful shutdown failed: %v", err)
		} else {
			log.Println("[MAIN] server shut down gracefully")
		}
	}()
	log.Println("listening on :5000")
	baseUrl := os.Getenv("BASE_IP_URL")
	log.Fatal(app.Listen(fmt.Sprintf("%s:5000", baseUrl)))
}
