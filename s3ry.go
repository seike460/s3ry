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
	"github.com/seike460/s3ry/internal/i18n"
	modernS3 "github.com/seike460/s3ry/internal/s3"
	"github.com/seike460/s3ry/internal/worker"
)

// S3ry Service Client Operator
type S3ry struct {
	Sess         *session.Session
	Svc          *s3.S3
	Bucket       string
	ModernClient *modernS3.Client // Modern S3 client for enhanced performance
	UseModern    bool             // Flag to enable modern backend
}

// ApNortheastOne Japan Region String
const ApNortheastOne = "ap-northeast-1"

// SelectBucketAndRegion get Region and Bucket
func SelectBucketAndRegion() (string, string) {

	// for Bucket Search
	s3ry := NewS3ry(ApNortheastOne)
	// show Bucket List & select
	buckets := s3ry.ListBuckets()
	selectBucket := s3ry.SelectItem(i18n.Sprintf("Which bucket do you use?"), buckets)
	ctx := context.Background()
	// Get bucket's region
	region, err := s3manager.GetBucketRegion(ctx, s3ry.Sess, selectBucket, ApNortheastOne)
	if err != nil {
		awsErrorPrint(err)
	}
	return region, selectBucket
}

// NewS3ry Create New S3ry struct
func NewS3ry(region string) *S3ry {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(region)},
	))
	svc := s3.New(sess)
	s := &S3ry{
		Sess:      sess,
		Svc:       svc,
		UseModern: false, // Default to legacy for backward compatibility
	}
	return s
}

// NewS3ryWithModernBackend Create New S3ry struct with modern backend enabled
func NewS3ryWithModernBackend(region string) *S3ry {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(region)},
	))
	svc := s3.New(sess)
	modernClient := modernS3.NewClient(region)

	s := &S3ry{
		Sess:         sess,
		Svc:          svc,
		ModernClient: modernClient,
		UseModern:    true,
	}
	return s
}

// EnableModernBackend enables the modern backend for better performance
func (s *S3ry) EnableModernBackend(region string) {
	if s.ModernClient == nil {
		s.ModernClient = modernS3.NewClient(region)
	}
	s.UseModern = true
}

// DisableModernBackend disables the modern backend (fallback to legacy)
func (s *S3ry) DisableModernBackend() {
	s.UseModern = false
}

// ListOperation return ListOperation for PromptItems
func (s S3ry) ListOperation() []PromptItems {
	items := []PromptItems{
		{Key: 0, Val: i18n.Sprintf("download")},
		{Key: 1, Val: i18n.Sprintf("upload")},
		{Key: 2, Val: i18n.Sprintf("delete object")},
		{Key: 3, Val: i18n.Sprintf("create object list")},
	}
	return items
}

// ListBuckets return ListBuckets for PromptItems
func (s S3ry) ListBuckets() []PromptItems {
	sps(i18n.Sprintf("Searching for buckets ..."))
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
	return items
}

// ListObjectsPages return ListObjectsPages for PromptItems
func (s S3ry) ListObjectsPages(bucket string) []PromptItems {
	sps(i18n.Sprintf("Searching for objects ..."))
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
	fmt.Println(i18n.Sprintf("Number of objects: "), len(items))
	return items
}

// GetObject get Object from S3 bucket
func (s S3ry) GetObject(bucket string, objectKey string) {
	if s.UseModern && s.ModernClient != nil {
		s.getObjectModern(bucket, objectKey)
	} else {
		s.getObjectLegacy(bucket, objectKey)
	}
}

// getObjectLegacy uses the original download method for backward compatibility
func (s S3ry) getObjectLegacy(bucket string, objectKey string) {
	sps(i18n.Sprintf("Downloading object ..."))
	filename := filepath.Base(objectKey)
	file, err := os.Create(filename)
	if err != nil {
		awsErrorPrint(err)
	}
	defer file.Close()
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
	fmt.Println(i18n.Sprintf("File downloaded,% s,% d bytes", filename, result))
}

// getObjectModern uses the modern S3 client with worker pool for enhanced performance
func (s S3ry) getObjectModern(bucket string, objectKey string) {
	sps(i18n.Sprintf("Downloading object (modern backend) ..."))
	filename := filepath.Base(objectKey)

	// Create modern downloader with enhanced configuration
	config := modernS3.DefaultDownloadConfig()
	config.ConcurrentDownloads = 3
	config.OnProgress = func(downloaded, total int64) {
		// Optional: show progress (basic implementation)
		if total > 0 {
			percent := float64(downloaded) / float64(total) * 100
			if percent == 100 {
				fmt.Printf("\r%s: %.1f%%", i18n.Sprintf("Progress"), percent)
			}
		}
	}

	downloader := modernS3.NewDownloader(s.ModernClient, config)
	defer downloader.Close()

	request := modernS3.DownloadRequest{
		Bucket:   bucket,
		Key:      objectKey,
		FilePath: filename,
	}

	ctx := context.Background()
	err := downloader.Download(ctx, request, config)
	if err != nil {
		// Fallback to legacy on error
		fmt.Printf("\n%s, %s\n", i18n.Sprintf("Modern backend failed"), i18n.Sprintf("falling back to legacy"))
		s.getObjectLegacy(bucket, objectKey)
		return
	}

	// Get file size for reporting
	fileInfo, err := os.Stat(filename)
	var fileSize int64
	if err == nil {
		fileSize = fileInfo.Size()
	}

	spe()
	fmt.Println(i18n.Sprintf("File downloaded (modern),% s,% d bytes", filename, fileSize))
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
	if s.UseModern && s.ModernClient != nil {
		s.uploadObjectModern(bucket, selectUpload)
	} else {
		s.uploadObjectLegacy(bucket, selectUpload)
	}
}

// uploadObjectLegacy uses the original upload method for backward compatibility
func (s S3ry) uploadObjectLegacy(bucket string, selectUpload string) {
	sps(i18n.Sprintf("Uploading object ..."))
	uploadObject := selectUpload
	uploader := s3manager.NewUploader(s.Sess)
	f, err := os.Open(uploadObject)
	if err != nil {
		awsErrorPrint(err)
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
	fmt.Println(i18n.Sprintf("Uploaded file,% s", uploadObject))
}

// uploadObjectModern uses the modern S3 client with worker pool for enhanced performance
func (s S3ry) uploadObjectModern(bucket string, selectUpload string) {
	sps(i18n.Sprintf("Uploading object (modern backend) ..."))
	uploadObject := selectUpload

	// Create modern uploader with enhanced configuration
	config := modernS3.DefaultUploadConfig()
	config.ConcurrentUploads = 3
	config.OnProgress = func(uploaded, total int64) {
		// Optional: show progress (basic implementation)
		if total > 0 {
			percent := float64(uploaded) / float64(total) * 100
			if percent == 100 {
				fmt.Printf("\r%s: %.1f%%", i18n.Sprintf("Progress"), percent)
			}
		}
	}

	uploader := modernS3.NewUploader(s.ModernClient, config)
	defer uploader.Close()

	request := modernS3.UploadRequest{
		Bucket:   bucket,
		Key:      uploadObject,
		FilePath: uploadObject,
	}

	ctx := context.Background()
	err := uploader.Upload(ctx, request, config)
	if err != nil {
		// Fallback to legacy on error
		fmt.Printf("\n%s, %s\n", i18n.Sprintf("Modern backend failed"), i18n.Sprintf("falling back to legacy"))
		s.uploadObjectLegacy(bucket, selectUpload)
		return
	}

	spe()
	fmt.Println(i18n.Sprintf("Uploaded file (modern),% s", uploadObject))
}

// SelectItem select PromptItems using promptui
func (s S3ry) SelectItem(label string, items []PromptItems) string {
	detail := "{{\"Selection Value\" | faint }} {{ .Val }}"

	for _, item := range items {
		if item.Tag == "Object" {
			detail = "{{\"Selection Value:\" | faint }} {{ .Val }}\n{{\"LastModified:\" | faint }} {{ .LastModified }}"
		}
		break
	}
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "->{{ .Val | red }}",
		Inactive: "{{ .Val | cyan }}",
		Selected: i18n.Sprintf("\"Selection Value:\" {{ .Val | red | cyan }}"),
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
		awsErrorPrint(err)
	}
	return items[i].Val
}

// DeleteObject delete Object from S3 bucket
func (s S3ry) DeleteObject(bucket string, item string) {
	if s.UseModern && s.ModernClient != nil {
		s.deleteObjectModern(bucket, item)
	} else {
		s.deleteObjectLegacy(bucket, item)
	}
}

// deleteObjectLegacy uses the original delete method for backward compatibility
func (s S3ry) deleteObjectLegacy(bucket string, item string) {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(item),
	}
	_, err := s.Svc.DeleteObject(input)
	if err != nil {
		awsErrorPrint(err)
	}
	fmt.Printf("File deleted")
}

// deleteObjectModern uses the modern S3 client with worker pool for enhanced performance
func (s S3ry) deleteObjectModern(bucket string, item string) {
	// Create a modern worker pool for delete operations
	config := worker.DefaultConfig()
	config.Workers = 3
	pool := worker.NewPool(config)
	pool.Start()
	defer pool.Stop()

	job := &worker.S3DeleteJob{
		Client: s.ModernClient,
		Bucket: bucket,
		Key:    item,
	}

	err := pool.Submit(job)
	if err != nil {
		// Fallback to legacy on error
		fmt.Printf("%s, %s\n", i18n.Sprintf("Modern backend failed"), i18n.Sprintf("falling back to legacy"))
		s.deleteObjectLegacy(bucket, item)
		return
	}

	// Wait for result
	select {
	case result := <-pool.Results():
		if result.Error != nil {
			// Fallback to legacy on error
			fmt.Printf("%s, %s\n", i18n.Sprintf("Modern backend failed"), i18n.Sprintf("falling back to legacy"))
			s.deleteObjectLegacy(bucket, item)
			return
		}
		fmt.Printf("File deleted (modern)")
	case <-time.After(30 * time.Second):
		// Timeout - fallback to legacy
		fmt.Printf("%s, %s\n", i18n.Sprintf("Modern backend timeout"), i18n.Sprintf("falling back to legacy"))
		s.deleteObjectLegacy(bucket, item)
	}
}

// SaveObjectList create S3 ObjectList And SaveList
func (s S3ry) SaveObjectList(bucket string) {
	items := S3ry.ListObjectsPages(s, bucket)
	t := time.Now()
	ObjectListFileName := "ObjectList-" + t.Format("2006-01-02-15-04-05") + ".txt"
	file, err := os.Create(ObjectListFileName)
	if err != nil {
		awsErrorPrint(err)
	}
	defer file.Close()
	for _, item := range items {
		_, err = file.Write(([]byte)("./" + item.Val + "," + strconv.FormatInt(item.Size, 10) + "\n"))
		if err != nil {
			awsErrorPrint(err)
		}
	}
	fmt.Println(i18n.Sprintf("Object list created:") + ObjectListFileName)
}

// Operations for Another package
// Operations maintains backward compatibility by using the legacy backend
func Operations(region string, bucket string) {
	OperationsWithBackend(region, bucket, false)
}

// OperationsWithBackend allows choosing between legacy and modern backend
func OperationsWithBackend(region string, bucket string, useModernBackend bool) {
	var s *S3ry
	if useModernBackend {
		s = NewS3ryWithModernBackend(region)
	} else {
		s = NewS3ry(region)
	}
	s.Bucket = bucket
	// show Bucket List & select
	operations := s.ListOperation()
	selectOperation := s.SelectItem(i18n.Sprintf("What are you doing?"), operations)

	switch selectOperation {
	case i18n.Sprintf("upload"):
		uploadItem := s.ListUpload(s.Bucket)
		selectUpload := s.SelectItem(i18n.Sprintf("Which file do you upload?"), uploadItem)
		s.UploadObject(s.Bucket, selectUpload)
	case i18n.Sprintf("create object list"):
		s.SaveObjectList(s.Bucket)
	case i18n.Sprintf("delete object"):
		items := s.ListObjectsPages(s.Bucket)
		item := s.SelectItem(i18n.Sprintf("Which files do you want to delete?"), items)
		s.DeleteObject(s.Bucket, item)
	default:
		// show Object List & select
		items := s.ListObjects(s.Bucket)
		selectObject := s.SelectItem(i18n.Sprintf("Which file do you want to download?"), items)
		// check File
		checkLocalExists(selectObject)
		// GetObject
		s.GetObject(s.Bucket, selectObject)
	}
}
