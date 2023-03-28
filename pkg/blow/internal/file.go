package internal

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/bingoohuang/berf/pkg/blow/internal/art"
	"github.com/bingoohuang/berf/pkg/util"
	"github.com/bingoohuang/gg/pkg/iox"
	"github.com/bingoohuang/gg/pkg/randx"
	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/bingoohuang/gg/pkg/uid"
	"github.com/bingoohuang/jj"
	"github.com/karrick/godirwalk"
	"github.com/mitchellh/go-homedir"
)

var filePathCache sync.Map

type DataItem struct {
	payload util.UploadPayload
}

func (d *DataItem) CreateFileField(fileFieldName string, uploadIndex bool) *util.Multipart {
	if uploadIndex {
		d.payload.Name = insertIndexToFilename(d.payload.Name)
	}

	return util.PrepareMultipartPayload(map[string]interface{}{
		fileFieldName: d.payload,
	})
}

var uploadIndexVal uint64

func insertIndexToFilename(name string) string {
	ext := filepath.Ext(name)
	if ext == "" {
		return fmt.Sprintf("%s.%d", name, atomic.AddUint64(&uploadIndexVal, 1))
	}

	name = strings.TrimSuffix(name, ext)
	return fmt.Sprintf("%s.%d%s", name, atomic.AddUint64(&uploadIndexVal, 1), ext)
}

type UploadChanValueType int

const (
	NormalFile UploadChanValueType = iota
	DirectBytes
)

type UploadChanValue struct {
	Data        func() *DataItem
	Path        string
	ContentType string
	Type        UploadChanValueType
}

func (v UploadChanValue) GetCachePath() string {
	prefix := fmt.Sprintf("%d-%s-", v.Type, v.ContentType)
	if v.Type == NormalFile {
		return prefix + v.Path
	}
	return prefix + "DirectBytes"
}

type FileReader interface {
	Read(cache bool) *UploadChanValue
	Start(ctx context.Context)
}

type fileReaders struct {
	readers      []FileReader
	currentIndex int
}

func (f *fileReaders) Read(cache bool) *UploadChanValue {
	value := f.readers[f.currentIndex].Read(cache)
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

func createDataItem(filePath string, isDiskFile bool, data []byte) func() *DataItem {
	var payload util.UploadPayload

	if isDiskFile {
		file, err := os.Open(filePath)
		if err != nil {
			log.Fatalf("open file %s failed: %v", filePath, err)
		}
		defer iox.Close(file)

		if stat, err := file.Stat(); err != nil {
			log.Fatalf("stat file: %s, error: %v", filePath, err)
		} else if stat.Size() <= 10<<20 /* 10 M*/ {
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, file)

			payload = util.UploadPayload{Val: buf.Bytes(), Name: filePath, Size: stat.Size()}
		} else {
			payload = util.UploadPayload{DiskFile: true, Val: []byte(filePath), Name: filePath, Size: stat.Size()}
		}
	} else {
		payload = util.UploadPayload{Val: data, Name: filePath, Size: int64(len(data))}
	}

	return func() *DataItem {
		return &DataItem{payload: payload}
	}
}

type artReader struct {
	uploadFileField string
	saveRandDir     string
}

func (r artReader) Read(cache bool) *UploadChanValue {
	uv := &UploadChanValue{
		Type:        DirectBytes,
		ContentType: "image/png",
	}

	cachePath := uv.GetCachePath()
	if cache {
		if load, ok := filePathCache.Load(cachePath); ok {
			return load.(*UploadChanValue)
		}
	}

	data := art.Random(".png")
	uv.Path = uid.New().String() + ".png"
	uv.Data = createDataItem(uv.Path, false, data)
	if r.saveRandDir != "" {
		util.LogErr1(os.WriteFile(filepath.Join(r.saveRandDir, uv.Path), data, os.ModePerm))
	}

	if cache {
		filePathCache.Store(cachePath, uv)
	}

	return uv
}

func (r artReader) Start(context.Context) {}

type randImgReader struct {
	uploadFileField string
	ContentType     string
	Extension       string
	saveRandDir     string
}

func (r randImgReader) Read(cache bool) *UploadChanValue {
	uv := &UploadChanValue{
		Type:        DirectBytes,
		ContentType: r.ContentType,
	}

	cachePath := uv.GetCachePath()
	if cache {
		if load, ok := filePathCache.Load(cachePath); ok {
			return load.(*UploadChanValue)
		}
	}

	c := randx.ImgConfig{Width: 640, Height: 320, RandomText: uid.New().String(), FastMode: false}
	data, _ := c.Gen(r.Extension)
	uv.Path = c.RandomText + r.Extension
	uv.Data = createDataItem(uv.Path, false, data)
	if r.saveRandDir != "" {
		util.LogErr1(os.WriteFile(filepath.Join(r.saveRandDir, uv.Path), data, os.ModePerm))
	}

	if cache {
		filePathCache.Store(cachePath, uv)
	}

	return uv
}

func (r randImgReader) Start(context.Context) {}

type randJsonReader struct {
	uploadFileField string
	saveRandDir     string
}

func (r randJsonReader) Read(cache bool) *UploadChanValue {
	uv := &UploadChanValue{
		Type:        DirectBytes,
		ContentType: "application/json; charset=utf-8",
	}
	cachePath := uv.GetCachePath()
	if cache {
		if load, ok := filePathCache.Load(cachePath); ok {
			return load.(*UploadChanValue)
		}
	}

	data := jj.Rand()
	uv.Path = uid.New().String() + ".json"
	uv.Data = createDataItem(uv.Path, false, data)
	if r.saveRandDir != "" {
		util.LogErr1(os.WriteFile(filepath.Join(r.saveRandDir, uv.Path), data, os.ModePerm))
	}
	if cache {
		filePathCache.Store(cachePath, uv)
	}

	return uv
}

func (r randJsonReader) Start(context.Context) {}

type dirReader struct {
	Dir             string
	ch              chan string
	uploadFileField string
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

func (f *dirReader) Read(cache bool) *UploadChanValue {
	fr := &fileReader{
		File:            <-f.ch,
		uploadFileField: f.uploadFileField,
	}
	return fr.Read(cache)
}

type fileReader struct {
	File            string
	uploadFileField string
}

func (f fileReader) Start(context.Context) {}

func (f fileReader) Read(cache bool) *UploadChanValue {
	uv := &UploadChanValue{
		Type:        NormalFile,
		ContentType: "",
		Path:        f.File,
	}
	if !cache {
		return uv
	}

	cachePath := uv.GetCachePath()
	if load, ok := filePathCache.Load(cachePath); ok {
		return load.(*UploadChanValue)
	}

	statSize := int64(0)
	if stat, err := os.Stat(uv.Path); err != nil {
		log.Fatalf("stat file: %s, error: %v", uv.Path, err)
	} else {
		statSize = stat.Size()
	}

	if statSize > 10<<20 /* 10 M*/ {
		uv.Data = createDataItem(uv.Path, true, nil)
	}
	filePathCache.Store(cachePath, uv)
	return uv
}

func CreateFileReader(uploadFileField, upload, saveRandDir string) FileReader {
	var rr fileReaders

	if saveRandDir != "" {
		if saveDir, err := os.Stat(saveRandDir); err != nil {
			log.Printf("stat saveRandDir %s, failed: %v", saveDir, err)
			saveRandDir = ""
		} else if !saveDir.IsDir() {
			log.Printf("saveRandDir %s is not a directory", saveDir)
			saveRandDir = ""
		}
	}

	uploadFiles := ss.Split(upload)
	for _, file := range uploadFiles {
		switch file {
		case "rand.art":
			rr.readers = append(rr.readers, &artReader{uploadFileField: uploadFileField, saveRandDir: saveRandDir})
		case "rand.png":
			rr.readers = append(rr.readers, &randImgReader{uploadFileField: uploadFileField, ContentType: "image/png", Extension: ".png", saveRandDir: saveRandDir})
		case "rand.jpg":
			rr.readers = append(rr.readers, &randImgReader{uploadFileField: uploadFileField, ContentType: "image/jpeg", Extension: ".jpeg", saveRandDir: saveRandDir})
		case "rand.json":
			rr.readers = append(rr.readers, &randJsonReader{uploadFileField: uploadFileField, saveRandDir: saveRandDir})
		default:
			file, _ = homedir.Expand(file)
			if stat, err := os.Stat(file); err != nil {
				log.Fatalf("stat upload %s failed: %v", file, err)
			} else if stat.IsDir() {
				rr.readers = append(rr.readers, &dirReader{Dir: file, uploadFileField: uploadFileField})
			} else {
				rr.readers = append(rr.readers, &fileReader{File: file, uploadFileField: uploadFileField})
			}
		}
	}

	return &rr
}
