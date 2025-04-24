package repository

import (
	"context"
	"ffmpeg-hls/entity"
	"fmt"
)

type VideoRepository interface {
	GetByID(ctx context.Context, id string) (*entity.Video, error)
}
type videoRepository struct{}

func NewVideoRepository() VideoRepository {
	return &videoRepository{}
}

func (r *videoRepository) GetByID(ctx context.Context, id string) (*entity.Video, error) {
	return &entity.Video{Dir: fmt.Sprintf("courses/%s", id)}, nil

}
