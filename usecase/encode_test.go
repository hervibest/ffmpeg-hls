package usecase

import (
	"context"
	"ffmpeg-hls/model"
	"ffmpeg-hls/util"
	"log"
	"os"
	"testing"

	"github.com/joho/godotenv"
)

func TestEncode(t *testing.T) {
	if err := godotenv.Load("../.env"); err != nil {
		panic(err)
	}

	serverKey := os.Getenv("HTTP_PROTOCOL") + os.Getenv("BASE_IP_URL") + ":" + os.Getenv("PORT")
	log.Print(serverKey)
	minio := util.InitMinio()
	req := &model.EncodeRequest{
		APIServer: serverKey,
		VideoID:   "sample-5s",
	}

	encodeUC := NewEncodeUseCase(minio)
	ctx := context.Background()
	err := encodeUC.EncodeAndUpload(ctx, req)
	if err != nil {
		log.Fatal(err)
	}
}
