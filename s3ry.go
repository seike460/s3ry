package s3ry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/manifoldco/promptui"
)

// TYPES

// S3ry Service Client Operator
type S3ry struct {
	sess   *session.Session
	svc    *s3.S3
	bucket string
}

// NewS3ry Create New S3ry struct
func NewS3ry() *S3ry {

	// for Bucket Search
	tempSess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String("ap-northeast-1")},
	))
	tempSvc := s3.New(tempSess)
	tempS3ry := &S3ry{
		sess: tempSess,
		svc:  tempSvc,
	}

	// for Bucket Search
	tempSess = session.Must(session.NewSession(&aws.Config{
		Region: aws.String("ap-northeast-1")},
	))
	tempSvc = s3.New(tempSess)
	tempS3ry = &S3ry{
		sess: tempSess,
		svc:  tempSvc,
	}

	// show Bucket List & select
	buckets := tempS3ry.ListBuckets()
	selectBucket := tempS3ry.SelectItem("どのバケットを利用しますか?", buckets)
	ctx := context.Background()

	region, err := s3manager.GetBucketRegion(ctx, tempSess, selectBucket, "ap-northeast-1")
	if err != nil {
		awsErrorPrint(err)
	}

	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(region)},
	))

	svc := s3.New(sess)

	s := &S3ry{
		sess:   sess,
		svc:    svc,
		bucket: selectBucket,
	}

	tempSess = nil
	tempSvc = nil
	tempS3ry = nil

	return s
}

// ListOperation return ListOperation for PromptItems
func (s S3ry) ListOperation() []PromptItems {
	items := []PromptItems{
		{Key: 0, Val: "ダウンロード"},
		{Key: 1, Val: "アップロード"},
		{Key: 2, Val: "オブジェクト削除"},
		{Key: 3, Val: "オブジェクトリスト"},
	}
	return items
}

// ListBuckets return ListBuckets for PromptItems
func (s S3ry) ListBuckets() []PromptItems {
	sps("バケットの検索中です...")
	input := &s3.ListBucketsInput{}
	listBuckets, err := s.svc.ListBuckets(input)
	if err != nil {
		awsErrorPrint(err)
	}
	items := []PromptItems{}
	for key, val := range listBuckets.Buckets {
		items = append(items, PromptItems{Key: key, Val: *val.Name, Tag: "Bucket"})
	}
	spe()
	return items
}

// ListObjectsPages return ListObjectsPages for PromptItems
func (s S3ry) ListObjectsPages(bucket string) []PromptItems {
	sps("オブジェクトの検索中です...")
	items := []PromptItems{}
	key := 0
	err := s.svc.ListObjectsPages(&s3.ListObjectsInput{Bucket: aws.String(bucket)},
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
	// @todo 並び替えオプションをフラグをグローバルにもたせて、KeyでもSort出来るようにする
	sort.Slice(items, func(i, j int) bool {
		return items[i].LastModified.After(items[j].LastModified)
	})
	spe()
	return items
}

// ListObjects return ListObjects for PromptItems
func (s S3ry) ListObjects(bucket string) []PromptItems {
	items := S3ry.ListObjectsPages(s, bucket)
	fmt.Println("オブジェクト数：", len(items))
	return items
}

// GetObject get Object from S3 bucket
func (s S3ry) GetObject(bucket string, objectKey string) {
	sps("オブジェクトのダウンロード中です...")
	filename := filepath.Base(objectKey)
	file, err := os.Create(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "")
		os.Exit(1)
	}
	defer file.Close()
	inputGet := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	}
	downloader := s3manager.NewDownloader(s.sess)
	result, err := downloader.Download(file, inputGet)
	if err != nil {
		if err := os.Remove(filename); err != nil {
			fmt.Println(err)
		}
		awsErrorPrint(err)
	}
	spe()
	fmt.Printf("ファイルをダウンロードしました, %s, %d bytes\n", filename, result)
}

// ListUpload return ListUpload for PromptItems
func (s S3ry) ListUpload(bucket string) []PromptItems {
	dir := dirwalk("")
	items := []PromptItems{}
	for key, val := range dir {
		items = append(items, PromptItems{Key: key, Val: val, Tag: "Upload"})
	}
	return items
}

// UploadObject put Object in S3 bucket
func (s S3ry) UploadObject(bucket string, selectUpload string) {
	sps("オブジェクトのアップロード中です...")
	uploadObject := selectUpload
	uploader := s3manager.NewUploader(s.sess)
	f, err := os.Open(uploadObject)
	if err != nil {
		fmt.Println(err)
		fmt.Fprintf(os.Stderr, "")
		os.Exit(1)
	}
	defer f.Close()

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

// SelectItem select PromptItems using promptui
func (s S3ry) SelectItem(label string, items []PromptItems) string {
	detail := `
{{ "選択値:" | faint }} {{ .Val }}
`
	for _, item := range items {
		if item.Tag == "Object" {
			detail = `
{{ "選択値:" | faint }} {{ .Val }}
{{ "最終更新日:" | faint }} {{ .LastModified }}
`
		}
		break
	}
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
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
		fmt.Printf("選択に失敗しました。終了します %v\n", err)
		os.Exit(0)
	}
	return items[i].Val
}

// DeleteObject delete Object from S3 bucket
func (s S3ry) DeleteObject(bucket string, item string) {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(item),
	}
	_, err := s.svc.DeleteObject(input)
	if err != nil {
		awsErrorPrint(err)
	}
	fmt.Printf("ファイルを削除しました")
}

// SaveObjectList create S3 ObjectList And SaveList
func (s S3ry) SaveObjectList(bucket string) {
	items := S3ry.ListObjectsPages(s, bucket)
	t := time.Now()
	ObjectListFileName := "ObjectList-" + t.Format("2006-01-02-15-04-05") + ".txt"
	file, err := os.Create(ObjectListFileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "")
		os.Exit(1)
	}
	defer file.Close()
	for _, item := range items {
		_, err = file.Write(([]byte)("./" + item.Val + "," + strconv.FormatInt(item.Size, 10) + "\n"))
		if err != nil {
			fmt.Println("書き込みエラーです", err)
			os.Exit(1)
		}
	}
	fmt.Println("オブジェクトリストを作成しました:" + ObjectListFileName)
}

// Operations for Another package
func Operations() {
	s := NewS3ry()
	// show Bucket List & select
	operations := s.ListOperation()
	selectOperation := s.SelectItem("何をしますか？", operations)

	switch selectOperation {
	case "アップロード":
		uploadItem := s.ListUpload(s.bucket)
		selectUpload := s.SelectItem("どのファイルをアップロードしますか?", uploadItem)
		s.UploadObject(s.bucket, selectUpload)
	case "オブジェクトリスト":
		s.SaveObjectList(s.bucket)
	case "オブジェクト削除":
		items := s.ListObjectsPages(s.bucket)
		item := s.SelectItem("どのファイルを削除しますか?", items)
		s.DeleteObject(s.bucket, item)
	default:
		// show Object List & select
		items := s.ListObjects(s.bucket)
		selectObject := s.SelectItem("どのファイルを取得しますか?", items)
		// check File
		CheckLocalExists(selectObject)
		// GetObject
		s.GetObject(s.bucket, selectObject)
	}
}
