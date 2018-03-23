package s3ry

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type s3ry struct {
	Sess *session.Session
	Svc  *s3.S3
}

func NewS3ry() *s3ry {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String("ap-northeast-1")},
	))

	svc := s3.New(sess)
	s := &s3ry{
		Sess: sess,
		Svc:  svc,
	}
	return s
}

func (s s3ry) ListBuckets() string {
	sps("バケットの検索中です...")
	input := &s3.ListBucketsInput{}
	listBuckets, err := s.Svc.ListBuckets(input)
	if err != nil {
		awsErrorPrint(err)
	}
	items := []PromptItems{}
	for key, val := range listBuckets.Buckets {
		items = append(items, PromptItems{Key: key, Val: *val.Name, Tag: "Bucket"})
	}
	spe()
	result := Run("どのバケットを利用しますか?", items)
	return result
}

func (s s3ry) ListObjects(bucket string) string {
	sps("オブジェクトの検索中です...")
	items := []PromptItems{}
	key := 0
	err := s.Svc.ListObjectsPages(&s3.ListObjectsInput{Bucket: aws.String(bucket)},
		func(listObjects *s3.ListObjectsOutput, lastPage bool) bool {
			for _, item := range listObjects.Contents {
				if strings.HasSuffix(*item.Key, "/") == false {
					items = append(items, PromptItems{Key: key, Val: *item.Key, LastModified: *item.LastModified, Tag: "Object"})
					key++
				}
			}
			return !lastPage
		})
	if err != nil {
		awsErrorPrint(err)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].LastModified.After(items[j].LastModified)
	})
	spe()
	result := Run("どのファイルを取得しますか?", items)
	return result
}

func (s s3ry) GetObject(bucket string, objectKey string) {
	sps("オブジェクトのダウンロード中です...")
	filename := filepath.Base(objectKey)
	file, err := os.Create(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "")
		os.Exit(1)
	}
	inputGet := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	}
	downloader := s3manager.NewDownloader(s.Sess)
	result, err := downloader.Download(file, inputGet)
	if err != nil {
		awsErrorPrint(err)
	}
	spe()
	fmt.Printf("ファイルをダウンロードしました, %s, %d bytes\n", filename, result)
}

func (s s3ry) UploadObject(bucket string) {
	dir := dirwalk()
	items := []PromptItems{}
	for key, val := range dir {
		items = append(items, PromptItems{Key: key, Val: val, Tag: "Bucket"})
	}
	selected := Run("どのファイルをアップロードしますか?", items)

	sps("オブジェクトのアップロード中です...")
	uploadObject := selected
	uploader := s3manager.NewUploader(s.Sess)
	f, err := os.Open(uploadObject)
	if err != nil {
		fmt.Fprintf(os.Stderr, "")
		os.Exit(1)
	}

	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(uploadObject),
		Body:   f,
	})
	if err != nil {
		awsErrorPrint(err)
	}
	spe()
	fmt.Printf("ファイルをアップロードしました, %s \n", uploadObject)
}
