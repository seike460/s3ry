package s3

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

// ListBuckets lists all buckets visible to the caller. ListBuckets is a
// global operation; each bucket's region is retained when the service returns
// it and is otherwise resolved lazily by BucketRegion.
func (s *Session) ListBuckets(ctx context.Context) ([]Bucket, error) {
	client := s.cache.clientForRegion(s.cfg.Region)

	var (
		buckets []Bucket
		token   *string
	)
	for {
		out, err := client.ListBuckets(ctx, &awss3.ListBucketsInput{
			ContinuationToken: token,
		})
		if err != nil {
			return nil, Classify("list-buckets", "", "", err)
		}

		for _, item := range out.Buckets {
			name := aws.ToString(item.Name)
			bucket := Bucket{
				Name: name,
			}
			if item.CreationDate != nil {
				bucket.CreationDate = *item.CreationDate
			}
			bucket.Region = aws.ToString(item.BucketRegion)
			if bucket.Region != "" {
				s.cache.cacheRegion(name, bucket.Region)
			}
			buckets = append(buckets, bucket)
		}

		nextToken := out.ContinuationToken
		if token != nil && nextToken != nil && aws.ToString(token) == aws.ToString(nextToken) {
			return nil, &Error{
				Kind: KindUnsupported,
				Op:   "list-buckets",
				Err:  errors.New("endpoint returned the same continuation token repeatedly"),
			}
		}

		token = nextToken
		if token == nil || aws.ToString(token) == "" {
			return buckets, nil
		}
	}
}
