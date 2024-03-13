package internal

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/bingoohuang/gg/pkg/osx"
)

type LogFile struct {
	File *os.File
	name string
	Pos  int64
	sync.Mutex
}

func (f *LogFile) WriteString(s string) {
	f.Lock()
	f.File.WriteString(s)
	f.Unlock()
}

func (f *LogFile) Write(b *bytes.Buffer) {
	f.Lock()
	b.WriteTo(f.File)
	f.Unlock()
}

func (f *LogFile) MarkPos() {
	f.Lock()
	f.Pos, _ = f.File.Seek(0, io.SeekCurrent)
	f.Unlock()
}

func CreateLogFile(verbose int, n int) *LogFile {
	if verbose < 2 && n != 1 {
		return nil
	}

	f, err := os.CreateTemp("", "blow_"+time.Now().Format(`20060102150405`)+"_"+"*.log")
	osx.ExitIfErr(err)

	fileName := f.Name()
	fmt.Printf("Log details to: %s\n", fileName)
	return &LogFile{File: f, name: fileName}
}

func (f *LogFile) GetLastLog() string {
	f.Lock()
	defer f.Unlock()

	data, _ := ReadFileFromPos(f.File, f.Pos)
	return string(data)
}

func (f *LogFile) Close() error {
	f.Lock()
	defer f.Unlock()

	return f.File.Close()
}

func (f *LogFile) Remove() error { return os.Remove(f.name) }

func ReadFileFromPos(f *os.File, pos int64) ([]byte, error) {
	_, err := f.Seek(pos, io.SeekStart)
	if err != nil {
		return nil, err
	}

	return io.ReadAll(f)
}

func ErrorIf(b bool, err1, err2 error) error {
	if b {
		return err1
	}

	return err2
}
