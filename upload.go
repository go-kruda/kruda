package kruda

import (
	"fmt"
	"io"
	"mime/multipart"
)

// FileUpload represents an uploaded file from a multipart form.
//
//	app := kruda.New(
//	    kruda.WithValidator(kruda.NewValidator()), // max_size/mime need a Validator
//	    kruda.WithBodyLimit(8<<20),                // BodyLimit defaults to 4MB
//	)
//
//	type UploadReq struct {
//	    Avatar *kruda.FileUpload `form:"avatar" validate:"required,max_size=5mb,mime=image/*"`
//	}
//
// BodyLimit is enforced by the transport before the handler runs, so a request
// larger than it gets a 413 and never reaches validation. A max_size above
// BodyLimit can therefore never fire; keep max_size under it, or raise it as
// above. max_size failures are reported as 422, BodyLimit as 413.
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
