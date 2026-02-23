package kruda

import (
	"fmt"
	"io"
	"mime/multipart"
)

// FileUpload represents an uploaded file from a multipart form.
// It provides metadata about the file and a method to read its content.
// Users reference this type as kruda.FileUpload in struct tags:
//
//	type UploadReq struct {
//	    Avatar *kruda.FileUpload `form:"avatar" validate:"required,max_size=5mb,mime=image/*"`
//	}
type FileUpload struct {
	// Name is the original filename as provided by the client.
	Name string

	// Size is the file size in bytes.
	Size int64

	// ContentType is the MIME type from the Content-Type header
	// of the multipart part (e.g. "image/png", "application/pdf").
	ContentType string

	// Header is the raw multipart file header for advanced use cases
	// such as accessing additional headers or the underlying temp file.
	Header *multipart.FileHeader
}

// Open opens the uploaded file for reading.
// Returns an io.ReadCloser — the caller must close it when done.
// Temp files are cleaned up by RemoveAll during request teardown,
// but closing promptly avoids holding file descriptors.
func (f *FileUpload) Open() (io.ReadCloser, error) {
	if f.Header == nil {
		return nil, fmt.Errorf("kruda: file upload header is nil")
	}
	file, err := f.Header.Open()
	if err != nil {
		return nil, fmt.Errorf("kruda: failed to open uploaded file %q: %w", f.Name, err)
	}
	return file, nil
}
