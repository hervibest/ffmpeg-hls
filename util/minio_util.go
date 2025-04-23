package util

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var minioClient *minio.Client

// Init initializes MinIO client
func InitMinio() {
	var (
		minioHost         = os.Getenv("MINIO_HOST")
		minioPort         = os.Getenv("MINIO_PORT")
		minioRootUser     = os.Getenv("MINIO_ROOT_USER")
		minioRootPassword = os.Getenv("MINIO_ROOT_PASSWORD")
		minioBucket       = os.Getenv("MINIO_TICKETS_BUCKET") // Bisa kamu ganti jadi lebih generik seperti `MINIO_BUCKET`
		minioLocation     = os.Getenv("MINIO_LOCATION")
		endpoint          = minioHost + ":" + minioPort
	)

	log.Print(minioHost)
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioRootUser, minioRootPassword, ""),
		Secure: false,
	})
	if err != nil {
		log.Fatalf("Failed to initialize MinIO client: %v", err)
	}

	minioClient = client

	// Make sure bucket exists
	exists, err := minioClient.BucketExists(context.Background(), minioBucket)
	if err != nil {
		log.Fatalf("Error checking bucket: %v", err)
	}

	if !exists {
		if err := minioClient.MakeBucket(context.Background(), minioBucket, minio.MakeBucketOptions{
			Region: minioLocation,
		}); err != nil {
			log.Fatalf("Failed to create bucket: %v", err)
		}
		log.Printf("Created bucket: %s", minioBucket)
	} else {
		log.Printf("Bucket already exists: %s", minioBucket)
	}
}

// UploadToS3 uploads file to MinIO bucket
func UploadToS3(ctx context.Context, bucket, objectName string, data []byte) error {
	reader := bytes.NewReader(data)
	_, err := minioClient.PutObject(ctx, bucket, objectName, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	if err != nil {
		return fmt.Errorf("failed to upload %s: %w", objectName, err)
	}
	return nil
}
