package main

import (
	"github.com/seike460/s3ry/s3ry"
)

func main() {
	s3ry := s3ry.NewS3ry()
	// show Bucket List & select
	selectBucket := s3ry.ListBuckets()
	// show Object List & select
	selectObject := s3ry.ListObjects(selectBucket)
	// check File
	s3ry.CheckLocalExists(selectObject)
	// GetObject
	s3ry.GetObject(selectBucket, selectObject)
}
