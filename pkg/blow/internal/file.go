package internal

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/bingoohuang/gg/pkg/randx"
	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/bingoohuang/gg/pkg/uid"
	"github.com/bingoohuang/jj"
	"github.com/karrick/godirwalk"
	"github.com/mitchellh/go-homedir"
)

type UploadChanValueType int

const (
	NormalFile UploadChanValueType = iota
	DirectBytes
)

type UploadChanValue struct {
	Type        UploadChanValueType
	Path        string
	Data        []byte
	ContentType string
}

func (v UploadChanValue) GetCachePath() string {
	prefix := fmt.Sprintf("%d-%s-", v.Type, v.ContentType)
	if v.Type == NormalFile {
		return prefix + v.Path
	}
	return prefix + "DirectBytes"
}

type FileReader interface {
	Read() UploadChanValue
	Start(ctx context.Context)
}

type fileReaders struct {
	readers      []FileReader
	currentIndex int
}

func (f *fileReaders) Read() UploadChanValue {
	value := f.readers[f.currentIndex].Read()
	if f.currentIndex++; f.currentIndex >= len(f.readers) {
		f.currentIndex = 0
	}

	return value
}

func (f fileReaders) Start(ctx context.Context) {
	for _, f := range f.readers {
		f.Start(ctx)
	}
}

type randPngReader struct{}

func (r randPngReader) Read() UploadChanValue {
	c := randx.ImgConfig{
		Width:      640,
		Height:     320,
		RandomText: uid.New().String(),
		FastMode:   false,
	}
	data, _ := c.Gen(".png")

	return UploadChanValue{
		Type:        DirectBytes,
		Data:        data,
		ContentType: "image/png",
		Path:        c.RandomText + ".png",
	}
}

func (r randPngReader) Start(context.Context) {}

type randJpgReader struct{}

func (r randJpgReader) Read() UploadChanValue {
	c := randx.ImgConfig{
		Width:      640,
		Height:     320,
		RandomText: uid.New().String(),
		FastMode:   false,
	}
	data, _ := c.Gen(".jpeg")

	return UploadChanValue{
		Type:        DirectBytes,
		Data:        data,
		ContentType: "image/jpeg",
		Path:        c.RandomText + ".jpeg",
	}
}

func (r randJpgReader) Start(context.Context) {}

type randJsonReader struct{}

func (r randJsonReader) Read() UploadChanValue {
	return UploadChanValue{
		Type:        DirectBytes,
		Data:        jj.Rand(),
		ContentType: "application/json; charset=utf-8",
		Path:        uid.New().String() + ".json",
	}
}

func (r randJsonReader) Start(context.Context) {}

type dirReader struct {
	Dir string
	ch  chan string
}

func (f *dirReader) Start(ctx context.Context) {
	f.ch = make(chan string, 1)
	errStop := fmt.Errorf("canceled")
	fn := func(osPathname string, dirEntry *godirwalk.Dirent) error {
		if v, e := dirEntry.IsDirOrSymlinkToDir(); v || e != nil {
			return e
		}

		if strings.HasPrefix(dirEntry.Name(), ".") {
			return nil
		}

		select {
		case <-ctx.Done():
			return errStop
		default:
			f.ch <- osPathname
		}

		return nil
	}

	options := godirwalk.Options{Unsorted: true, Callback: fn}
	go func() {
		defer close(f.ch)

		for {
			if err := godirwalk.Walk(f.Dir, &options); err != nil {
				log.Printf("walk dir: %s error: %v", f.Dir, err)
				return
			}
		}
	}()
}

func (f *dirReader) Read() UploadChanValue {
	file := <-f.ch
	return CreateNormalUploadChanValue(file)
}

func CreateNormalUploadChanValue(file string) UploadChanValue {
	return UploadChanValue{
		Type:        NormalFile,
		Data:        []byte(file),
		ContentType: "",
		Path:        file,
	}
}

type fileReader struct {
	File string
}

func (f fileReader) Start(context.Context) {}

func (f fileReader) Read() UploadChanValue {
	return UploadChanValue{
		Type:        NormalFile,
		ContentType: "",
		Path:        f.File,
	}
}

func CreateFileReader(upload string) FileReader {
	var rr fileReaders

	uploadFiles := ss.Split(upload)
	for _, file := range uploadFiles {
		switch file {
		case "rand.png":
			rr.readers = append(rr.readers, &randPngReader{})
		case "rand.jpg":
			rr.readers = append(rr.readers, &randJpgReader{})
		case "rand.json":
			rr.readers = append(rr.readers, &randJsonReader{})
		default:
			file, _ = homedir.Expand(file)
			if stat, err := os.Stat(file); err != nil {
				log.Fatalf("stat upload %s failed: %v", file, err)
			} else if stat.IsDir() {
				rr.readers = append(rr.readers, &dirReader{Dir: file})
			} else {
				rr.readers = append(rr.readers, &fileReader{File: file})
			}
		}
	}

	return &rr
}
