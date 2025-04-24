package handler

import (
	"ffmpeg-hls/model"
	"ffmpeg-hls/usecase"
	"ffmpeg-hls/worker"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
)

type EncodeHandler interface {
	UploadVideo(ctx *fiber.Ctx) error
}

type encodeHandler struct {
	encodeUseCase usecase.EncodeUseCase
	encodeWorker  worker.EncodeWorker
}

func NewEncodeHandler(encodeUseCase usecase.EncodeUseCase, encodeWorker worker.EncodeWorker) EncodeHandler {
	return &encodeHandler{
		encodeUseCase: encodeUseCase,
		encodeWorker:  encodeWorker,
	}
}

func (h *encodeHandler) UploadVideo(ctx *fiber.Ctx) error {
	video, err := ctx.FormFile("video")
	if err != nil {
		log.Printf("[CLIENT ERROR] [UPLOAD VIDEO] from video error : %v", err)
		return fiber.NewError(http.StatusUnprocessableEntity, "Invalid video file, make sure you have ti provide the required video")
	}

	cwd, err := os.Getwd()
	if err != nil {
		log.Printf("failed to get current directory: %v", err)
		return fiber.NewError(http.StatusInternalServerError, "internal error")
	}

	savePath := filepath.Join(cwd, "usecase", "tmp", video.Filename)
	if err := ctx.SaveFile(video, savePath); err != nil {
		log.Printf("[INTERNAL ERROR] [UPLOAD VIDEO] save video error : %v", err)
		return fiber.NewError(http.StatusServiceUnavailable, "Something wrong please try again later.")
	}

	serverKey := os.Getenv("HTTP_PROTOCOL") + os.Getenv("BASE_IP_URL") + ":" + os.Getenv("PORT")
	encodeRequest := &model.EncodeRequest{
		APIServer: serverKey,
		VideoID:   video.Filename,
	}

	go func() {
		h.encodeWorker.SendJobToWorker(encodeRequest)
	}()

	return ctx.Status(http.StatusOK).JSON(fiber.Map{
		"Success": true,
	})

}
