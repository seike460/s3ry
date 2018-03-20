package s3Searcher

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/briandowns/spinner"
	"github.com/manifoldco/promptui"
)

var sp = spinner.New(spinner.CharSets[34], 100*time.Millisecond)

func spS(label string) {
	fmt.Println(label)
	sp.Start()
}

func spE() {
	sp.Stop()
}

type S3Searcher struct {
	Sess *session.Session
	Svc  *s3.S3
}

type PromptItems struct {
	Key          int
	Val          string
	LastModified time.Time
	Tag          string
}

func NewS3Searcher() *S3Searcher {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String("ap-northeast-1")},
	))

	svc := s3.New(sess)
	S3Searcher := &S3Searcher{
		Sess: sess,
		Svc:  svc,
	}
	return S3Searcher
}

func (s3Searcher S3Searcher) ListBuckets() string {
	spS("バケットの検索中です...")
	input := &s3.ListBucketsInput{}
	listBuckets, err := s3Searcher.Svc.ListBuckets(input)
	if err != nil {
		awsErrorPrint(err)
	}
	Items := []PromptItems{}
	for key, val := range listBuckets.Buckets {
		Items = append(Items, PromptItems{Key: key, Val: *val.Name, Tag: "Bucket"})
	}
	spE()
	result := Run("どのバケットを利用しますか?", Items)
	return *listBuckets.Buckets[result].Name
}
func (s3Searcher S3Searcher) ListObjects(bucket string) string {
	spS("オブジェクトの検索中です...")
	Items := []PromptItems{}
	key := 0
	err := s3Searcher.Svc.ListObjectsPages(&s3.ListObjectsInput{Bucket: aws.String(bucket)},
		func(listObjects *s3.ListObjectsOutput, lastPage bool) bool {
			for _, item := range listObjects.Contents {
				if strings.HasSuffix(*item.Key, "/") == false {
					Items = append(Items, PromptItems{Key: key, Val: *item.Key, LastModified: *item.LastModified, Tag: "Object"})
					key++
				}
			}
			return !lastPage
		})
	if err != nil {
		awsErrorPrint(err)
	}
	sort.Slice(Items, func(i, j int) bool {
		return Items[i].LastModified.After(Items[j].LastModified)
	})
	spE()
	result := Run("どのファイルを取得しますか?", Items)
	return Items[result].Val
}

func (s3Searcher S3Searcher) CheckLocalExists(objectKey string) {
	filename := filepath.Base(objectKey)
	_, err := os.Stat(filename)
	if err == nil {
		var overlide string
		fmt.Printf("そのファイルは存在します。上書きしますか ファイル名:%s, [Yy]/[Nn])\n", filename)
		fmt.Scan(&overlide)
		if overlide != "y" && overlide != "Y" {
			fmt.Println("終了します")
			os.Exit(0)
		}
	}
}

func (s3Searcher S3Searcher) GetObject(bucket string, objectKey string) {
	spS("オブジェクトのダウンロード中です...")
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
	downloader := s3manager.NewDownloader(s3Searcher.Sess)
	result, err := downloader.Download(file, inputGet)
	if err != nil {
		awsErrorPrint(err)
	}
	spE()
	fmt.Printf("ファイルをダウンロードしました, %s, %d bytes\n", filename, result)
}

func (s3Searcher S3Searcher) UploadObject(bucket string) {

	dir := dirwalk()
	Items := []PromptItems{}
	for key, val := range dir {
		Items = append(Items, PromptItems{Key: key, Val: val, Tag: "Bucket"})
	}
	selected := Run("どのファイルをアップロードしますか?", Items)

	spS("オブジェクトのアップロード中です...")
	uploadObject := Items[selected].Val
	uploader := s3manager.NewUploader(s3Searcher.Sess)
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
	spE()
	fmt.Printf("ファイルをアップロードしました, %s \n", uploadObject)
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

func Run(label string, items []PromptItems) int {
	detail := `
{{ "選択値:" | faint }} {{ .Val }}
`
	if items[0].Tag == "Object" {
		detail = `
{{ "選択値:" | faint }} {{ .Val }}
{{ "最終更新日:" | faint }} {{ .LastModified }}
`
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
		os.Exit(1)
	}
	return i
}

func dirwalk() []string {
	dir, _ := os.Getwd()
	files, _ := ioutil.ReadDir(dir)
	var paths []string
	for _, file := range files {
		if !file.IsDir() {
			paths = append(paths, file.Name())
		}
	}
	return paths
}
