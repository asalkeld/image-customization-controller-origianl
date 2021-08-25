package imagehandler

import (
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/openshift/assisted-image-service/pkg/isoeditor"
)

// imageFileSystem is an http.FileSystem that creates a virtual filesystem of
// host images. These *could* be later cached as real files.
type imageFileSystem struct {
	isoFile string
	baseURL string
	images  []*imageFile
	mu      *sync.Mutex
	log     logr.Logger
}

type ImageFileServer interface {
	FileSystem() http.FileSystem
	ServerImage(name string, ignitionContent []byte) (string, error)
}

var _ ImageFileServer = &imageFileSystem{}
var _ http.FileSystem = &imageFileSystem{}

func NewImageFileServer(logger logr.Logger, isoFile, baseURL string) ImageFileServer {
	return &imageFileSystem{
		log:     logger,
		isoFile: isoFile,
		baseURL: baseURL,
		images:  []*imageFile{},
		mu:      &sync.Mutex{},
	}
}

func NotImplementedFn(name string) error { return fmt.Errorf("%s not implemented", name) }

func (f *imageFileSystem) FileSystem() http.FileSystem {
	return f
}

func (f *imageFileSystem) ServerImage(name string, ignitionContent []byte) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.images = append(f.images, &imageFile{
		name:            name,
		ignitionContent: ignitionContent,
	})

	return path.Join(f.baseURL, name), nil
}

func (f *imageFileSystem) imageFileByName(name string) *imageFile {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, im := range f.images {
		if im.name == name {
			return im
		}
	}
	return nil
}

// file interface implementation

var _ fs.File = &imageFile{}

func (f *imageFileSystem) Readdir(n int) ([]fs.FileInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := []fs.FileInfo{}
	for _, im := range f.images {
		result = append(result, im)
	}
	return result, nil
}

func (f *imageFileSystem) Open(name string) (http.File, error) {
	f.log.Info("Open", "path", name)
	if name == "/" {
		return f, nil
	}
	// if we need caching and it is cached, return the real file here
	im := f.imageFileByName(path.Base(name))
	if im == nil {
		return nil, fs.ErrNotExist
	}
	var err error
	im.rhcosStreamReader, err = isoeditor.NewRHCOSStreamReader(f.isoFile, im.ignitionContent)
	if err != nil {
		f.log.Error(err, "creating isoeditor.NewRHCOSStreamReader")
		return nil, err
	}
	return im, nil
}

func (f *imageFileSystem) Close() error                      { return nil }
func (f *imageFileSystem) Stat() (fs.FileInfo, error)        { return fs.FileInfo(f), nil }
func (f *imageFileSystem) Read(p []byte) (n int, err error)  { return 0, NotImplementedFn("Read") }
func (f *imageFileSystem) Write(p []byte) (n int, err error) { return 0, NotImplementedFn("Write") }
func (f *imageFileSystem) Seek(offset int64, whence int) (int64, error) {
	return 0, NotImplementedFn("Seek")
}

// fileInfo interface implementation

var _ fs.FileInfo = &imageFileSystem{}

func (i *imageFileSystem) Name() string       { return "/" }
func (i *imageFileSystem) Size() int64        { return 0 }
func (i *imageFileSystem) Mode() fs.FileMode  { return 0755 }
func (i *imageFileSystem) ModTime() time.Time { return time.Now() }
func (i *imageFileSystem) IsDir() bool        { return true }
func (i *imageFileSystem) Sys() interface{}   { return nil }
