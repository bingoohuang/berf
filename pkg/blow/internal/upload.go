package internal

import (
	"bytes"
	"context"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"sync"

	"github.com/bingoohuang/gg/pkg/iox"
)

func DealUploadFilePath(ctx context.Context, uploadReader FileReader, postFileCh chan UploadChanValue) {
	defer close(postFileCh)

	uploadReader.Start(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			postFileCh <- uploadReader.Read()
		}
	}
}

var filePathCache sync.Map

type cacheItem struct {
	data    []byte
	headers map[string]string
}

// ReadMultipartFile read file filePath for upload in multipart,
// return multipart content, form data content type and error.
func ReadMultipartFile(cache bool, fieldName string, uv UploadChanValue) (
	data io.Reader, dataSize int, headers map[string]string, err error,
) {
	if cache {
		if load, ok := filePathCache.Load(uv.GetCachePath()); ok {
			item := load.(cacheItem)
			return bytes.NewReader(item.data), len(item.data), item.headers, nil
		}
	}

	statSize := int64(0)

	if uv.Type == NormalFile {
		if stat, err := os.Stat(uv.Path); err != nil {
			log.Fatalf("stat file: %s, error: %v", uv.Path, err)
		} else {
			statSize = stat.Size()
		}

		if statSize > 10<<20 /* 10 M*/ {
			file, err := os.Open(uv.Path)
			if err != nil {
				return nil, 0, nil, err
			}
			payload := PrepareMultipartPayload(map[string]interface{}{
				fieldName: &PayloadFile{ReadCloser: file, Name: uv.Path, Size: statSize},
			})

			return payload.body, int(payload.size), payload.headers, nil
		}
	}

	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	part, err := writer.CreateFormFile(fieldName, filepath.Base(uv.Path))
	if err != nil {
		return nil, 0, nil, err
	}
	if uv.Type == NormalFile {
		file, err := os.Open(uv.Path)
		if err != nil {
			return nil, 0, nil, err
		}
		_, _ = io.Copy(part, file)
		iox.Close(file)
	} else {
		part.Write(uv.Data)
	}
	iox.Close(writer)

	item := cacheItem{data: buffer.Bytes(), headers: map[string]string{
		"Content-Type": writer.FormDataContentType(),
	}}

	if cache {
		filePathCache.Store(uv.GetCachePath(), item)
	}

	return bytes.NewReader(item.data), len(item.data), item.headers, nil
}

// OpenFile opens file successfully or panic.
func OpenFile(f string) *os.File {
	r, err := os.Open(f)
	if err != nil {
		panic(err)
	}

	return r
}
