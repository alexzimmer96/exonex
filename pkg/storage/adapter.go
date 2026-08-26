package storage

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Adapter struct {
	bucket           string
	client           *s3.Client
	presignClient    *s3.PresignClient
	credentialsCache aws.CredentialsCache
}

func NewAdapter(endpoint, region, bucket, accessKey, secretKey string) *Adapter {
	staticProvider := credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")
	client := s3.New(s3.Options{
		BaseEndpoint: new(endpoint),
		Credentials:  staticProvider,
		Region:       region,
		UsePathStyle: true,
	})
	return &Adapter{
		bucket:        bucket,
		client:        client,
		presignClient: s3.NewPresignClient(client),
	}
}

// =====================================================================================================================

type CreatePresignedUploadURLAction struct {
	Key        string    `json:"key"`
	Expiration time.Time `json:"expiration"`
}

// CreatePresignedUploadURL creates an URL for the S3 Provider that can be used to upload a new object.
func (a *Adapter) CreatePresignedUploadURL(ctx context.Context, action CreatePresignedUploadURLAction) (string, error) {
	response, err := a.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: new(a.bucket),
		Key:    new(action.Key),
	})
	if err != nil {
		return "", err
	}
	return response.URL, nil
}
