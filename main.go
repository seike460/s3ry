package main

import (
	"github.com/seike460/s3ry/s3ry"
)

func main() {
	s := s3ry.NewS3ry()
	// show Bucket List & select
	operations := s.ListOperation()
	selectOperation := s.SelectItem("何をしますか？", operations)
	// show Bucket List & select
	buckets := s.ListBuckets()
	selectBucket := s.SelectItem("どのバケットを利用しますか?", buckets)

	switch selectOperation {
	case "アップロード":
		uploadItem := s.ListUpload(selectBucket)
		selectUpload := s.SelectItem("どのファイルをアップロードしますか?", uploadItem)
		s.UploadObject(selectBucket, selectUpload)
	case "オブジェクトリスト":
		s.SaveObjectList(selectBucket)
	case "オブジェクト削除":
		items := s.ListObjectsPages(selectBucket)
		item := s.SelectItem("どのファイルを削除しますか?", items)
		s.DeleteObject(selectBucket, item)
	default:
		// show Object List & select
		selectObject := s.ListObjects(selectBucket)
		// check File
		s3ry.CheckLocalExists(selectObject)
		// GetObject
		s.GetObject(selectBucket, selectObject)
	}
}
