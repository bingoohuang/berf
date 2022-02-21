package internal

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/karrick/godirwalk"
	"github.com/mitchellh/go-homedir"
)

func DealUploadFilePath(ctx context.Context, uploadFilepath string, postFileCh chan string) {
	if uploadFilepath == "" {
		return
	}

	uploadFilepath, _ = homedir.Expand(uploadFilepath)
	fs, err := os.Stat(uploadFilepath)
	if err != nil && os.IsNotExist(err) {
		log.Fatalf("%s dos not exist", uploadFilepath)
	}
	if err != nil {
		log.Fatalf("stat file %s error  %v", uploadFilepath, err)
	}

	defer close(postFileCh)

	if !fs.IsDir() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				postFileCh <- uploadFilepath
			}
		}
	}

	errStop := fmt.Errorf("canceled")
	fn := func(osPathname string, directoryEntry *godirwalk.Dirent) error {
		if v, e := directoryEntry.IsDirOrSymlinkToDir(); v || e != nil {
			return e
		}

		if strings.HasPrefix(directoryEntry.Name(), ".") {
			return nil
		}

		postFileCh <- osPathname
		select {
		case <-ctx.Done():
			return errStop
		default:
			postFileCh <- osPathname
		}

		return nil
	}
	options := godirwalk.Options{Unsorted: true, Callback: fn}

	for {
		if err := godirwalk.Walk(uploadFilepath, &options); err != nil {
			log.Printf("walk dir: %s error: %v", uploadFilepath, err)
			return
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
func ReadMultipartFile(nocache bool, fieldName, filePath string) (data io.Reader, dataSize int, headers map[string]string, err error) {
	if !nocache {
		if load, ok := filePathCache.Load(filePath); ok {
			item := load.(cacheItem)
			return bytes.NewReader(item.data), len(item.data), item.headers, nil
		}
	}

	file := OpenFile(filePath)

	statSize := int64(0)
	if stat, err := file.Stat(); err != nil {
		log.Fatalf("stat file: %s, error: %v", filePath, err)
	} else {
		statSize = stat.Size()
	}

	if statSize <= 10<<20 /* 10 M*/ {
		defer file.Close()

		var buffer bytes.Buffer
		writer := multipart.NewWriter(&buffer)

		part, err := writer.CreateFormFile(fieldName, filepath.Base(filePath))
		if err != nil {
			return nil, 0, nil, err
		}
		_, _ = io.Copy(part, file)
		_ = writer.Close()

		item := cacheItem{data: buffer.Bytes(), headers: map[string]string{
			"Content-Type": writer.FormDataContentType(),
		}}

		if !nocache {
			filePathCache.Store(filePath, item)
		}

		return bytes.NewReader(item.data), len(item.data), item.headers, nil
	}

	payload := PrepareMultipartPayload(map[string]interface{}{
		fieldName: &PayloadFile{ReadCloser: file, Name: file.Name(), Size: statSize},
	})

	return payload.body, int(payload.size), payload.headers, nil
}

// OpenFile opens file successfully or panic.
func OpenFile(f string) *os.File {
	r, err := os.Open(f)
	if err != nil {
		panic(err)
	}

	return r
}
