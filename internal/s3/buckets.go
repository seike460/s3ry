package s3

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
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

		buckets = s.collectBuckets(buckets, out.Buckets)

		token, err = nextListToken(token, out.ContinuationToken)
		if err != nil {
			return nil, err
		}
		if aws.ToString(token) == "" {
			return buckets, nil
		}
	}
}

// collectBuckets converts API bucket entries, caching each returned region.
func (s *Session) collectBuckets(buckets []Bucket, items []s3types.Bucket) []Bucket {
	for _, item := range items {
		bucket := Bucket{
			Name:   aws.ToString(item.Name),
			Region: aws.ToString(item.BucketRegion),
		}
		if item.CreationDate != nil {
			bucket.CreationDate = *item.CreationDate
		}
		if bucket.Region != "" {
			s.cache.cacheRegion(bucket.Name, bucket.Region)
		}
		buckets = append(buckets, bucket)
	}
	return buckets
}

// nextListToken returns the next continuation token, detecting an endpoint
// that repeats the same token forever.
func nextListToken(prev, next *string) (*string, error) {
	if prev != nil && next != nil && aws.ToString(prev) == aws.ToString(next) {
		return nil, &Error{
			Kind: KindUnsupported,
			Op:   "list-buckets",
			Err:  errors.New("endpoint returned the same continuation token repeatedly"),
		}
	}
	return next, nil
}
