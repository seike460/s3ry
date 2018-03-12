package main

import (
	"github.com/seike460/s3Searcher/s3Searcher"
)

func main() {
	s3Searcher := s3Searcher.NewS3Searcher()
	// show Bucket List & select
	selectBucket := s3Searcher.ListBuckets()
	// show Object List & select
	selectObject := s3Searcher.ListObjects(selectBucket)
	// check File
	s3Searcher.CheckLocalExists(selectObject)
	// GetObject
	s3Searcher.GetObject(selectBucket, selectObject)
}
