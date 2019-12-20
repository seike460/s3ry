package main

import (
	"github.com/seike460/s3ry"
)

func main() {
	region, selectBucket := s3ry.SelectBucketAndRegion()
	s3ry.Operations(region, selectBucket)
}
