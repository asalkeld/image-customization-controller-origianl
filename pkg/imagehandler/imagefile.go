package imagehandler

import (
	"io"
	"io/fs"
	"time"
)

// imageFile is the http.File use in imageFileSystem.
type imageFile struct {
	io.ReadSeekCloser
	name              string
	size              int64
	ignitionContent   []byte
	rhcosStreamReader io.ReadSeeker
}

// file interface implementation

var _ fs.File = &imageFile{}

func (f *imageFile) Read(p []byte) (n int, err error) {
	return f.rhcosStreamReader.Read(p)
}

func (f *imageFile) Seek(offset int64, whence int) (int64, error) {
	return f.rhcosStreamReader.Seek(offset, whence)
}

func (f *imageFile) Write(p []byte) (n int, err error)        { return 0, NotImplementedFn("Write") }
func (f *imageFile) Stat() (fs.FileInfo, error)               { return fs.FileInfo(f), nil }
func (f *imageFile) Close() error                             { return nil }
func (f *imageFile) Readdir(count int) ([]fs.FileInfo, error) { return []fs.FileInfo{}, nil }

// fileInfo interface implementation

var _ fs.FileInfo = &imageFile{}

func (i *imageFile) Name() string       { return i.name }
func (i *imageFile) Size() int64        { return i.size }
func (i *imageFile) Mode() fs.FileMode  { return 0444 }
func (i *imageFile) ModTime() time.Time { return time.Now() }
func (i *imageFile) IsDir() bool        { return false }
func (i *imageFile) Sys() interface{}   { return nil }
