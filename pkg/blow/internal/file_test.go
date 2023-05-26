package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChangeUploadName(t *testing.T) {
	setUploadFileChanger(".%i%ext")
	name := changeUploadName("/Users/bingoo/Downloads/20230523194821.27.jpg")
	assert.Equal(t, "/Users/bingoo/Downloads/20230523194821.27.1.jpg", name)

	setUploadFileChanger("%clear%i%ext")
	name = changeUploadName("/Users/bingoo/Downloads/20230523194821.27.jpg")
	assert.Equal(t, "1.jpg", name)
}
