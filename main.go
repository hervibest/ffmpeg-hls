// main.go
package main

import (
	"ffmpeg-hls/handler"
	"ffmpeg-hls/repository"
	"ffmpeg-hls/usecase"
	"ffmpeg-hls/util"
	"fmt"
	"log"
	"os"

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

	_ = handler.NewEncodeHandler(encodeUC)
	videoHandler := handler.NewVideoHandler(videoUC)

	app := fiber.New()

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	app.Get("/videos/:videoID/playlists/:playlist", videoHandler.VideoManifest)
	app.Get("/videos/:videoID/keys/:key", videoHandler.VideoKey)

	log.Println("listening on :5000")
	baseUrl := os.Getenv("BASE_IP_URL")
	log.Fatal(app.Listen(fmt.Sprintf("%s:5000", baseUrl)))
}
