package internal

import (
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"mime"
	"path/filepath"
	"strings"
)

// MultipartPayload is the multipart payload.
type MultipartPayload struct {
	headers map[string]string
	body    io.Reader
	size    int64
}

func (m *MultipartPayload) Close() error {
	if c, ok := m.body.(io.Closer); ok {
		return c.Close()
	}

	return nil
}

// PayloadFileReader is the interface which means a reader which represents a file.
type PayloadFileReader interface {
	io.Reader

	FileName() string
	FileSize() int64
}

// PayloadFile means the file payload.
type PayloadFile struct {
	io.ReadCloser

	Name string
	Size int64
}

// FileName returns the filename
func (p PayloadFile) FileName() string { return p.Name }

// FileSize returns the file size.
func (p PayloadFile) FileSize() int64 { return p.Size }

const (
	crlf = "\r\n"
)

// PrepareMultipartPayload prepares the multipart playload of http request.
// Multipart request has the following structure:
//  POST /upload HTTP/1.1
//  Other-Headers: ...
//  Content-Type: multipart/form-data; boundary=$boundary
//  \r\n
//  --$boundary\r\n    👈 request body starts here
//  Content-Disposition: form-data; name="field1"\r\n
//  Content-Type: text/plain; charset=utf-8\r\n
//  Content-Length: 4\r\n
//  \r\n
//  $content\r\n
//  --$boundary\r\n
//  Content-Disposition: form-data; name="field2"\r\n
//  ...
//  --$boundary--\r\n
// https://stackoverflow.com/questions/39761910/how-can-you-upload-files-as-a-stream-in-go/39781706
// https://blog.depa.do/post/bufferless-multipart-post-in-go
func PrepareMultipartPayload(fields map[string]interface{}) *MultipartPayload {
	var buf [8]byte
	if _, err := io.ReadFull(rand.Reader, buf[:]); err != nil {
		panic(err)
	}
	boundary := fmt.Sprintf("%x", buf[:])
	totalSize := 0
	headers := map[string]string{
		"Content-Type": fmt.Sprintf("multipart/form-data; boundary=%s", boundary),
	}

	parts := make([]io.Reader, 0)

	fieldBoundary := "--" + boundary + crlf
	str := strings.NewReader

	for k, v := range fields {
		if v == nil {
			continue
		}

		parts = append(parts, str(fieldBoundary))
		totalSize += len(fieldBoundary)

		switch vf := v.(type) {
		case string:
			header := fmt.Sprintf(`Content-Disposition: form-data; name="%s"`, k)
			parts = append(parts, str(header+crlf+crlf), str(v.(string)), str(crlf))
			totalSize += len(header) + len(crlf+crlf) + len(v.(string)) + len(crlf)
		case io.Reader:
			if fr, ok := vf.(PayloadFileReader); ok {
				fileName := fr.FileName()
				header := strings.Join([]string{
					fmt.Sprintf(`Content-Disposition: form-data; name="%s"; filename="%s"`, k, filepath.Base(fileName)),
					fmt.Sprintf(`Content-Type: %s`, mime.TypeByExtension(filepath.Ext(fileName))),
					fmt.Sprintf(`Content-Length: %d`, fr.FileSize()),
				}, crlf)
				parts = append(parts, str(header+crlf+crlf), vf, str(crlf))
				totalSize += len(header) + len(crlf+crlf) + int(fr.FileSize()) + len(crlf)
			} else {
				log.Printf("Ignore unsupported multipart payload type %t", v)
			}
		default:
			log.Printf("Ignore unsupported multipart payload type %t", v)
		}
	}

	finishBoundary := "--" + boundary + "--" + crlf
	parts = append(parts, str(finishBoundary))
	totalSize += len(finishBoundary)
	headers["Content-Length"] = fmt.Sprintf("%d", totalSize)

	return &MultipartPayload{headers: headers, body: io.MultiReader(parts...), size: int64(totalSize)}
}
