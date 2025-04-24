package handler

import (
	"bytes"
	"ffmpeg-hls/model"
	"ffmpeg-hls/usecase"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type VideoHandler interface {
	VideoKey(ctx *fiber.Ctx) error
	VideoManifest(ctx *fiber.Ctx) error
}

type videoHandler struct {
	videoUseCase usecase.VideoUseCase
}

func NewVideoHandler(videoUseCase usecase.VideoUseCase) VideoHandler {
	return &videoHandler{videoUseCase: videoUseCase}
}

func (h *videoHandler) VideoManifest(ctx *fiber.Ctx) error {
	videoID := ctx.Params("videoID")
	playlist := ctx.Params("playlist") // e.g. "360p.m3u8" or "master.m3u8"

	request := &model.VideoManifestRequest{
		VideoID:  videoID,
		Playlist: playlist,
	}

	response, err := h.videoUseCase.VideoManifest(ctx.Context(), request)
	if err != nil {
		return err
	}

	ctx.Type("application/vnd.apple.mpegurl", "utf-8")
	return ctx.SendString(strings.Join(response, "\n"))
}

func (h *videoHandler) VideoKey(ctx *fiber.Ctx) error {
	videoID := ctx.Params("videoID")
	key := ctx.Params("key") // e.g. "360p.m3u8" or "master.m3u8"

	request := &model.VideoKeyRequest{
		VideoID: videoID,
		KeyName: key,
	}

	response, err := h.videoUseCase.VideoKey(ctx.Context(), request)
	if err != nil {
		return err
	}

	ctx.Type("application/octet-stream")
	ctx.Response().Header.Set("Content-Length", fmt.Sprintf("%d", len(response)))
	return ctx.SendStream(bytes.NewReader(response)) // or c.Send(data) if prefered
}
