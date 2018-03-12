package s3Searcher

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/manifoldco/promptui"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type S3Searcher struct {
	Sess *session.Session
	Svc  *s3.S3
}

type ListItems struct {
	Key          int
	Val          string
	LastModified time.Time
}

func NewS3Searcher() *S3Searcher {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("ap-northeast-1")},
	)
	if err != nil {
		awsErrorPrint(err)
	}
	svc := s3.New(sess)
	S3Searcher := &S3Searcher{
		Sess: sess,
		Svc:  svc,
	}
	return S3Searcher
}

func (s3Searcher S3Searcher) ListBuckets() string {
	input := &s3.ListBucketsInput{}
	listBuckets, err := s3Searcher.Svc.ListBuckets(input)
	if err != nil {
		awsErrorPrint(err)
	}
	Items := []ListItems{}
	for key, val := range listBuckets.Buckets {
		Items = append(Items, ListItems{Key: key, Val: *val.Name})
	}
	result := Run("どのBucketsを利用しますか？", Items)
	return *listBuckets.Buckets[result].Name
}
func (s3Searcher S3Searcher) ListObjects(bucket string) string {
	listObjects, err := s3Searcher.Svc.ListObjects(&s3.ListObjectsInput{Bucket: aws.String(bucket)})
	if err != nil {
		awsErrorPrint(err)
	}
	Items := []ListItems{}
	for key, item := range listObjects.Contents {
		if strings.HasSuffix(*item.Key, "/") == false {
			Items = append(Items, ListItems{Key: key, Val: *item.Key, LastModified: *item.LastModified})
		}
	}
	result := Run("どのファイルを取得しますか？", Items)
	return *listObjects.Contents[result].Key
}

func (s3Searcher S3Searcher) CheckLocalExists(objectKey string) {
	_, err := os.Stat(objectKey)
	if err == nil {
		var overlide string
		fmt.Printf("そのファイルは存在します。上書きしますか？ ファイル名:%s, [Yy]/[Nn])\n", objectKey)
		fmt.Scan(&overlide)
		if overlide != "y" && overlide != "Y" {
			fmt.Println("終了します")
			os.Exit(0)
		}
	}
}

func (s3Searcher S3Searcher) GetObject(bucket string, objectKey string) {
	filename := filepath.Base(objectKey)
	file, err := os.Create(filename)
	if err != nil {
		fmt.Errorf("failed to create file %q, %v", filename, err)
		os.Exit(1)
	}
	inputGet := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	}
	downloader := s3manager.NewDownloader(s3Searcher.Sess)
	result, err := downloader.Download(file, inputGet)
	if err != nil {
		awsErrorPrint(err)
	}
	fmt.Printf("ファイルをダウンロードしました, %s, %d bytes\n", filename, result)
}
func awsErrorPrint(err error) {
	if aerr, ok := err.(awserr.Error); ok {
		fmt.Println(aerr.Error())
	} else {
		fmt.Println(err.Error())
	}
	fmt.Println(os.Stderr)
	os.Exit(1)
}

func Run(label string, items []ListItems) int {
	detail := `
{{ "選択値:" | faint }} {{ .Val }}
`
	if items[0].LastModified.After(time.Date(1900, 1, 1, 1, 0, 0, 123456789, time.Local)) {
		detail = `
{{ "選択値:" | faint }} {{ .Val }}
{{ "最終更新日:" | faint }} {{ .LastModified }}
`
	}

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}?",
		Active:   "->{{ .Val | red }}",
		Inactive: "{{ .Val | cyan }}",
		Selected: "選択値 {{ .Val | red | cyan }}",
		Details:  detail,
	}

	searcher := func(input string, index int) bool {
		item := items[index]
		name := strings.Replace(strings.ToLower(item.Val), " ", "", -1)
		input = strings.Replace(strings.ToLower(input), " ", "", -1)
		return strings.Contains(name, input)
	}

	prompt := promptui.Select{
		Label:     label,
		Items:     items,
		Templates: templates,
		Size:      20,
		Searcher:  searcher,
	}

	i, _, err := prompt.Run()

	if err != nil {
		fmt.Printf("プロンプトの選択に失敗しました、終了します %v\n", err)
		os.Exit(1)
	}
	return i
}
