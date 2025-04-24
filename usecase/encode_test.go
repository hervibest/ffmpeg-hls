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

	minio := util.InitMinio()

	req := EncodeRequest{
		APIServer: "http://192.168.0.115:5000",
		VideoID:   "Profil Kandidat Emas PROPER 2021 _ PT. Pertamina Hulu Mahakam - South Processing Unit (SPU)",
	}

	ctx := context.Background()
	err := EncodeAndUpload(ctx, req, minio)
	if err != nil {
		log.Fatal(err)
	}
}
