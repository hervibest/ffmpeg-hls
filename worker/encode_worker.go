package worker

import (
	"context"
	"ffmpeg-hls/model"
	"ffmpeg-hls/usecase"
	"log"
)

type EncodeWorker interface {
	Run(ctx context.Context)
	SendJobToWorker(request *model.EncodeRequest)
}

type encodeWorker struct {
	jobs          chan *model.EncodeRequest
	workerNumber  int
	encodeUseCase usecase.EncodeUseCase
}

func NewEncodeWorker(workerNumber int, encodeUseCase usecase.EncodeUseCase) EncodeWorker {
	jobs := make(chan *model.EncodeRequest, 2)
	return &encodeWorker{
		jobs:          jobs,
		workerNumber:  workerNumber,
		encodeUseCase: encodeUseCase,
	}
}

func (w *encodeWorker) Run(ctx context.Context) {
	// Jalankan worker-worker paralel
	for i := 0; i < w.workerNumber; i++ {
		go func(workerID int) {
			for request := range w.jobs {
				log.Printf("[WORKER #%d] Received job for video: %s", workerID, request.VideoID)
				if err := w.encodeUseCase.EncodeAndUpload(context.TODO(), request); err != nil {
					log.Printf("[WORKER #%d][ERROR] Failed to encode and upload: %v", workerID, err)
				}
			}
			log.Printf("[WORKER #%d] Channel closed, exiting", workerID)
		}(i)
	}

	// Goroutine untuk memonitor ctx.Done()
	go func() {
		<-ctx.Done()
		log.Println("[WORKER MANAGER] context cancelled, closing job channel")
		close(w.jobs)
	}()
}

func (w *encodeWorker) SendJobToWorker(request *model.EncodeRequest) {
	w.jobs <- request
}
