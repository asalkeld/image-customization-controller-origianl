package imagehandler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestImageHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/host-xyz-45.qcow", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	imageServer := &imageFileSystem{
		log:         zap.New(zap.UseDevMode(true)),
		isoFile:     "dummyfile.iso",
		isoFileSize: 12345,
		baseURL:     "http://localhost:8080",
		images: []*imageFile{
			{
				name:              "host-xyz-45.qcow",
				size:              12345,
				ignitionContent:   []byte("asietonarst"),
				rhcosStreamReader: strings.NewReader("aiosetnarsetin"),
			},
		},
		mu: &sync.Mutex{},
	}

	handler := http.FileServer(imageServer.FileSystem())
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the response body is what we expect.
	expected := `aiosetnarsetin`
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}
