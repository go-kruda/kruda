package kruda

import (
	"fmt"
	"io"
	"mime/multipart"
)

// FileUpload represents an uploaded file from a multipart form.
//
//	type UploadReq struct {
//	    Avatar *kruda.FileUpload `form:"avatar" validate:"required,max_size=5mb,mime=image/*"`
//	}
type FileUpload struct {
	Name        string
	Size        int64
	ContentType string
	Header      *multipart.FileHeader // raw header for advanced use cases
}

// Open opens the uploaded file for reading. The caller must close it when done.
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
