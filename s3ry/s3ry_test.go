package s3ry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewS3ry(t *testing.T) {
	s := NewS3ry()
	assert.Equal(t, s, s, "they should be equal")
	assert.NotEqual(t, s, 460, "they should not be equal")
	assert.NotNil(t, s)
	CheckLocalExists("main.go")
	//s.ListObjectsPages("seike460")
}
