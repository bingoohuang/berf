package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChangeUploadName(t *testing.T) {
	t.Setenv("UPLOAD_INDEX", ".%i")
	name := changeUploadName("/Users/bingoo/Downloads/20230523194821.27.jpg")
	assert.Equal(t, "/Users/bingoo/Downloads/20230523194821.27.1.jpg", name)
}
