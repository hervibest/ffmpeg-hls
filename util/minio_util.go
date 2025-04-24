package util

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Minio struct {
	minioClient *minio.Client
	buckeName   string
}

// Init initializes MinIO client
func InitMinio() *Minio {
	var (
		minioHost         = os.Getenv("BASE_IP_URL")
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

	// Make sure bucket exists
	exists, err := client.BucketExists(context.Background(), minioBucket)
	if err != nil {
		log.Fatalf("Error checking bucket: %v", err)
	}

	if !exists {
		if err := client.MakeBucket(context.Background(), minioBucket, minio.MakeBucketOptions{
			Region: minioLocation,
		}); err != nil {
			log.Fatalf("Failed to create bucket: %v", err)
		}
		log.Printf("Created bucket: %s", minioBucket)
	} else {
		log.Printf("Bucket already exists: %s", minioBucket)
	}

	return &Minio{
		minioClient: client,
		buckeName:   minioBucket,
	}
}

// UploadToS3 uploads file to MinIO bucket
func (u *Minio) UploadToS3(ctx context.Context, bucket, objectName string, data []byte) error {
	reader := bytes.NewReader(data)
	_, err := u.minioClient.PutObject(ctx, bucket, objectName, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	if err != nil {
		return fmt.Errorf("failed to upload %s: %w", objectName, err)
	}
	return nil
}

func (u *Minio) GetBucketName() string {
	return u.buckeName
}

func (u *Minio) GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error) {
	object, err := u.minioClient.GetObject(ctx, bucketName, objectName, opts)
	if err != nil {
		return nil, err
	}

	return object, nil
}

func (u *Minio) PresignedGetObject(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error) {
	object, err := u.minioClient.PresignedGetObject(ctx, bucketName, objectName, expires, reqParams)
	if err != nil {
		return nil, err
	}

	return object, nil
}
