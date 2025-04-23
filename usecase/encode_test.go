package usecase

import (
	"context"
	"ffmpeg-hls/util"
	"log"
	"testing"

	"github.com/joho/godotenv"
)

func TestEncode(t *testing.T) {
	if err := godotenv.Load("../.env"); err != nil {
		panic(err)
	}

	util.InitMinio()

	req := EncodeRequest{
		InputPath: "tmp/input.mp4",
		OutputDir: "tmp/output/video1234",
		S3Prefix:  "courses/123/video1234",
		Bucket:    "ffmpeg",

		APIServer: "http://localhost:5000",
		VideoID:   "video1234",
	}

	ctx := context.Background()
	err := EncodeAndUpload(ctx, req)
	if err != nil {
		log.Fatal(err)
	}
}
