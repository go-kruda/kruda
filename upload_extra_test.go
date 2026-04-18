package kruda

import (
	"testing"
)

// --- FileUpload.Open success path ---

func TestFileUpload_Open_WithContent(t *testing.T) {
	fh := createTestFileHeader(t, "file", "test.txt", "text/plain", []byte("hello"))
	fu := &FileUpload{
		Name:        "test.txt",
		Size:        5,
		ContentType: "text/plain",
		Header:      fh,
	}
	rc, err := fu.Open()
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	rc.Close()
}
