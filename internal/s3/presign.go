package s3

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

const maxPresignExpires = 7 * 24 * time.Hour

// PresignGet returns a presigned URL for downloading one object.
func (s *Session) PresignGet(ctx context.Context, bucket, key string, expires time.Duration) (string, error) {
	if err := ValidateBucketName(bucket); err != nil {
		return "", Classify("presign", bucket, key, err)
	}
	if err := ValidateKey(key); err != nil {
		return "", Classify("presign", bucket, key, err)
	}
	if s.Options().NoSignRequest {
		return "", &Error{
			Kind:   KindInvalid,
			Op:     "presign",
			Bucket: bucket,
			Key:    key,
			Err:    errors.New("presigned URLs require credentials (remove --no-sign-request)"),
		}
	}
	if expires < time.Second || expires > maxPresignExpires {
		return "", &Error{
			Kind:   KindInvalid,
			Op:     "presign",
			Bucket: bucket,
			Key:    key,
			Err:    errors.New("presigned URL expiry must be between 1 second and 7 days"),
		}
	}

	client, err := s.client(ctx, bucket)
	if err != nil {
		return "", Classify("presign", bucket, key, err)
	}

	presigned, err := awss3.NewPresignClient(client).PresignGetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, awss3.WithPresignExpires(expires))
	if err != nil {
		return "", Classify("presign", bucket, key, err)
	}
	return presigned.URL, nil
}
