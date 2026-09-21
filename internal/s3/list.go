package s3

import (
	"context"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const defaultListMaxKeys int32 = 1000

// ListPage lists one page of objects and common prefixes under prefix.
func (s *Session) ListPage(ctx context.Context, bucket, prefix, token string, maxKeys int32) (*Page, error) {
	if err := ValidateBucketName(bucket); err != nil {
		return nil, err
	}

	client, err := s.client(ctx, bucket)
	if err != nil {
		return nil, Classify("list", bucket, prefix, err)
	}

	out, err := client.ListObjectsV2(ctx, listObjectsInput(bucket, prefix, token, maxKeys, aws.String("/")))
	if err != nil {
		return nil, Classify("list", bucket, prefix, err)
	}

	return pageFromListOutput(bucket, prefix, out), nil
}

// Stat returns the metadata for one object.
func (s *Session) Stat(ctx context.Context, bucket, key string) (*ObjectInfo, error) {
	if err := ValidateBucketName(bucket); err != nil {
		return nil, err
	}
	if err := ValidateKey(key); err != nil {
		return nil, err
	}

	client, err := s.client(ctx, bucket)
	if err != nil {
		return nil, Classify("stat", bucket, key, err)
	}

	out, err := client.HeadObject(ctx, &awss3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, Classify("stat", bucket, key, err)
	}

	return &ObjectInfo{
		Object: Object{
			Key:          key,
			Size:         aws.ToInt64(out.ContentLength),
			LastModified: aws.ToTime(out.LastModified),
			ETag:         trimETag(out.ETag),
			StorageClass: string(out.StorageClass),
		},
		ContentType: aws.ToString(out.ContentType),
		Metadata:    copyMetadata(out.Metadata),
		VersionID:   aws.ToString(out.VersionId),
	}, nil
}

// Open starts a GetObject request and returns its response body.
func (s *Session) Open(ctx context.Context, bucket, key, byteRange string) (io.ReadCloser, error) {
	if err := ValidateBucketName(bucket); err != nil {
		return nil, err
	}
	if err := ValidateKey(key); err != nil {
		return nil, err
	}

	client, err := s.client(ctx, bucket)
	if err != nil {
		return nil, Classify("get", bucket, key, err)
	}

	out, err := client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Range:  optionalListString(byteRange),
	})
	if err != nil {
		return nil, Classify("get", bucket, key, err)
	}
	if out.Body == nil {
		return nil, &Error{Kind: KindUnknown, Op: "get"}
	}

	return out.Body, nil
}

func listObjectsInput(bucket, prefix, token string, maxKeys int32, delimiter *string) *awss3.ListObjectsV2Input {
	if maxKeys <= 0 {
		maxKeys = defaultListMaxKeys
	}
	return &awss3.ListObjectsV2Input{
		Bucket:            aws.String(bucket),
		Prefix:            optionalListString(prefix),
		Delimiter:         delimiter,
		MaxKeys:           aws.Int32(maxKeys),
		ContinuationToken: optionalListString(token),
	}
}

func optionalListString(value string) *string {
	if value == "" {
		return nil
	}
	return aws.String(value)
}

func pageFromListOutput(bucket, prefix string, out *awss3.ListObjectsV2Output) *Page {
	page := &Page{
		Bucket:      bucket,
		Prefix:      prefix,
		Prefixes:    make([]string, 0, len(out.CommonPrefixes)),
		Objects:     make([]Object, 0, len(out.Contents)),
		NextToken:   aws.ToString(out.NextContinuationToken),
		IsTruncated: aws.ToBool(out.IsTruncated),
	}
	for _, item := range out.CommonPrefixes {
		page.Prefixes = append(page.Prefixes, aws.ToString(item.Prefix))
	}
	for _, item := range out.Contents {
		page.Objects = append(page.Objects, objectFromListObject(item))
	}
	return page
}

func objectFromListObject(item s3types.Object) Object {
	return Object{
		Key:          aws.ToString(item.Key),
		Size:         aws.ToInt64(item.Size),
		LastModified: aws.ToTime(item.LastModified),
		ETag:         trimETag(item.ETag),
		StorageClass: string(item.StorageClass),
	}
}

func trimETag(value *string) string {
	etag := aws.ToString(value)
	etag = strings.TrimPrefix(etag, `"`)
	etag = strings.TrimSuffix(etag, `"`)
	return etag
}

func copyMetadata(metadata map[string]string) map[string]string {
	copied := make(map[string]string, len(metadata))
	for key, value := range metadata {
		copied[key] = value
	}
	return copied
}
