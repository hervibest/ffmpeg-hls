package handler

import (
	"ffmpeg-hls/model"
	"ffmpeg-hls/usecase"
	"log"
	"net/http"

	"github.com/gofiber/fiber/v2"
)

type EncodeHandler interface {
}

type encodeHandler struct {
	encodeUseCase usecase.EncodeUseCase
}

func NewEncodeHandler(encodeUseCase usecase.EncodeUseCase) EncodeHandler {
	return &encodeHandler{
		encodeUseCase: encodeUseCase,
	}
}

func (h *encodeHandler) UploadVideo(ctx *fiber.Ctx) error {
	video, err := ctx.FormFile("video")
	if err != nil {
		log.Printf("[CLIENT ERROR] [UPLOAD VIDEO] from video error : %v", err)
		return fiber.NewError(http.StatusUnprocessableEntity, "Invalid video file, make sure you have ti provide the required video")
	}

	if err := ctx.SaveFile(video, "tmp"); err != nil {
		log.Printf("[INTERNAL ERROR] [UPLOAD VIDEO] save video error : %v", err)
		return fiber.NewError(http.StatusServiceUnavailable, "Something wrong please try again later.")
	}

	go func() {
		encodeRequest := &model.EncodeRequest{
			APIServer: "http://192.168.0.115:5000",
			VideoID:   video.Filename,
		}
		if err := h.encodeUseCase.EncodeAndUpload(ctx.Context(), encodeRequest); err != nil {
			log.Printf("[INTERNAL ERROR] [UPLOAD VIDEO] encode and upload video error : %v", err)
		}
	}()

	return ctx.Status(http.StatusOK).JSON(fiber.Map{
		"Success": true,
	})

}
