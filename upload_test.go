package kruda

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/textproto"
	"testing"
)

// createTestFileHeader builds a real multipart.FileHeader by writing a
// multipart body and parsing it back — no mocks needed.
func createTestFileHeader(t *testing.T, fieldName, fileName, contentType string, content []byte) *multipart.FileHeader {
	t.Helper()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="`+fieldName+`"; filename="`+fileName+`"`)
	h.Set("Content-Type", contentType)

	part, err := w.CreatePart(h)
	if err != nil {
		t.Fatalf("create part: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write part: %v", err)
	}
	w.Close()

	reader := multipart.NewReader(&buf, w.Boundary())
	form, err := reader.ReadForm(1 << 20)
	if err != nil {
		t.Fatalf("read form: %v", err)
	}

	files := form.File[fieldName]
	if len(files) == 0 {
		t.Fatalf("no files found for field %q", fieldName)
	}
	return files[0]
}

func TestFileUploadStructFields(t *testing.T) {
	fu := FileUpload{
		Name:        "avatar.png",
		Size:        12345,
		ContentType: "image/png",
		Header:      nil,
	}

	if fu.Name != "avatar.png" {
		t.Errorf("Name = %q, want %q", fu.Name, "avatar.png")
	}
	if fu.Size != 12345 {
		t.Errorf("Size = %d, want %d", fu.Size, 12345)
	}
	if fu.ContentType != "image/png" {
		t.Errorf("ContentType = %q, want %q", fu.ContentType, "image/png")
	}
	if fu.Header != nil {
		t.Error("Header should be nil")
	}
}

func TestFileUploadOpenSuccess(t *testing.T) {
	content := []byte("hello world file content")
	fh := createTestFileHeader(t, "avatar", "test.txt", "text/plain", content)

	fu := &FileUpload{
		Name:        fh.Filename,
		Size:        fh.Size,
		ContentType: "text/plain",
		Header:      fh,
	}

	rc, err := fu.Open()
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("content = %q, want %q", got, content)
	}
}

func TestFileUploadOpenNilHeader(t *testing.T) {
	fu := &FileUpload{
		Name:        "test.txt",
		Size:        100,
		ContentType: "text/plain",
		Header:      nil,
	}

	rc, err := fu.Open()
	if rc != nil {
		t.Error("expected nil reader for nil Header")
	}
	if err == nil {
		t.Fatal("expected error for nil Header")
	}

	want := "kruda: file upload header is nil"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}
