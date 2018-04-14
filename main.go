package main

import (
	"github.com/seike460/s3ry/s3ry"
)

func main() {
	s := s3ry.NewS3ry()
	// show Bucket List & select
	selectOperation := s.SelectOperation()
	// show Bucket List & select
	selectBucket := s.ListBuckets()
	switch selectOperation {
	case "アップロード":
		s.UploadObject(selectBucket)
	case "オブジェクトリスト":
		s.SaveObjectList(selectBucket)
	case "オブジェクト削除":
		s.DeleteObject(selectBucket)
	default:
		// show Object List & select
		selectObject := s.ListObjects(selectBucket)
		// check File
		s3ry.CheckLocalExists(selectObject)
		// GetObject
		s.GetObject(selectBucket, selectObject)
	}
}
