package util

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

// Multipart is the multipart Payload.
type Multipart struct {
	Headers map[string]string
	parts   []interface{}
	Size    int64
}

type osFileReader struct {
	*os.File
}

func (f *osFileReader) Read(b []byte) (n int, err error) {
	n, err = f.File.Read(b)

	if err == io.EOF {
		if err2 := f.File.Close(); err2 != nil {
			log.Fatalf("close file failed: %v", err2)
		}
	}

	return n, err
}

func (m Multipart) NewReader() io.Reader {
	var readers []io.Reader

	for _, part := range m.parts {
		switch v := part.(type) {
		case []byte:
			readers = append(readers, bytes.NewReader(v))
		case string:
			readers = append(readers, strings.NewReader(v))
		case UploadPayload:
			if v.DiskFile {
				filename := string(v.Val)
				file, err := os.Open(filename)
				if err != nil {
					log.Fatalf("open %s failed: %v", filename, err)
				}
				readers = append(readers, &osFileReader{File: file})
			} else {
				readers = append(readers, bytes.NewReader(v.Val))
			}
		default:
			log.Fatalf("bad format: %T", part)
		}
	}

	return io.MultiReader(readers...)
}

// UploadPayload means the file Payload.
type UploadPayload struct {
	Val      []byte
	Name     string
	Size     int64
	DiskFile bool
}

// FileName returns the filename
func (p UploadPayload) FileName() string { return p.Name }

// FileSize returns the file Size.
func (p UploadPayload) FileSize() int64 { return p.Size }

const (
	crlf = "\r\n"
)

// PrepareMultipartPayload prepares the multipart playload of http request.
// Multipart request has the following structure:
//
//	POST /upload HTTP/1.1
//	Other-Headers: ...
//	Content-Type: multipart/form-Data; boundary=$boundary
//	\r\n
//	--$boundary\r\n    👈 request body starts here
//	Content-Disposition: form-Data; name="field1"\r\n
//	Content-Type: text/plain; charset=utf-8\r\n
//	Content-Length: 4\r\n
//	\r\n
//	$content\r\n
//	--$boundary\r\n
//	Content-Disposition: form-Data; name="field2"\r\n
//	...
//	--$boundary--\r\n
//
// https://stackoverflow.com/questions/39761910/how-can-you-upload-files-as-a-stream-in-go/39781706
// https://blog.depa.do/post/bufferless-multipart-post-in-go
func PrepareMultipartPayload(fields map[string]interface{}) *Multipart {
	var buf [8]byte
	if _, err := io.ReadFull(rand.Reader, buf[:]); err != nil {
		panic(err)
	}
	boundary := fmt.Sprintf("%x", buf[:])
	totalSize := 0
	headers := map[string]string{
		"Content-Type": fmt.Sprintf("multipart/form-Data; boundary=%s", boundary),
	}

	parts := make([]interface{}, 0)

	fieldBoundary := "--" + boundary + crlf

	for k, v := range fields {
		if v == nil {
			continue
		}

		parts = append(parts, fieldBoundary)
		totalSize += len(fieldBoundary)

		switch vf := v.(type) {
		case string:
			header := fmt.Sprintf(`Content-Disposition: form-Data; name="%s"`, k)
			part := header + crlf + crlf + v.(string) + crlf
			parts = append(parts, part)
			totalSize += len(part)
		case UploadPayload:
			fileName := vf.FileName()
			header := strings.Join([]string{
				fmt.Sprintf(`Content-Disposition: form-Data; name="%s"; filename="%s"`, k, filepath.Base(fileName)),
				fmt.Sprintf(`Content-Type: %s`, mime.TypeByExtension(filepath.Ext(fileName))),
				fmt.Sprintf(`Content-Length: %d`, vf.FileSize()),
			}, crlf)
			parts = append(parts, header+crlf+crlf, vf, crlf)
			totalSize += len(header) + len(crlf+crlf) + int(vf.FileSize()) + len(crlf)
		default:
			log.Printf("Ignore unsupported multipart Payload type %T", v)
		}
	}

	finishBoundary := "--" + boundary + "--" + crlf
	parts = append(parts, finishBoundary)
	totalSize += len(finishBoundary)
	headers["Content-Length"] = fmt.Sprintf("%d", totalSize)

	return &Multipart{Headers: headers, parts: parts, Size: int64(totalSize)}
}
