package s3ry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewS3ry(t *testing.T) {
	s := NewS3ry()
	operations := s.ListOperation()
	buckets := s.ListBuckets()
	s.UploadObject("seike460-gotest", "testUploadFile")
	uploadItem := s.ListUpload("seike460-gotest")
	s.SaveObjectList("seike460-gotest")
	items := s.ListObjectsPages("seike460-gotest")
	selectObject := s.ListObjects("seike460-gotest")
	s.DeleteObject("seike460-gotest", "testUploadFile")
	CheckLocalExists("testNothingsFile")
	assert.NotNil(t, operations)
	assert.NotNil(t, buckets)
	assert.NotNil(t, uploadItem)
	assert.NotNil(t, items)
	assert.NotNil(t, selectObject)
}
