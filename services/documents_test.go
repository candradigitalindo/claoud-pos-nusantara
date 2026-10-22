package services

import (
	"archive/zip"
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"testing"
)

func documentTestFile(t *testing.T, name string, content []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	p, err := w.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	p.Write(content)
	w.Close()
	r := httptest.NewRequest("POST", "/", &body)
	r.Header.Set("Content-Type", w.FormDataContentType())
	if err := r.ParseMultipartForm(1024 * 1024); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.MultipartForm.RemoveAll() })
	return r.MultipartForm.File["file"][0]
}

func TestDocumentFormatsAndSize(t *testing.T) {
	for _, extension := range []string{"docx", "xlsx", "pptx"} {
		t.Run(extension, func(t *testing.T) {
			var content bytes.Buffer
			w := zip.NewWriter(&content)
			for _, name := range []string{"[Content_Types].xml", map[string]string{"docx": "word/document.xml", "xlsx": "xl/workbook.xml", "pptx": "ppt/presentation.xml"}[extension]} {
				f, _ := w.Create(name)
				f.Write([]byte("<xml/>"))
			}
			w.Close()
			header := documentTestFile(t, "uji."+extension, content.Bytes())
			file, err := header.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			if _, err := documentContentType(file, header.Size, header.Filename); err != nil {
				t.Fatal(err)
			}
			if _, err := documentContentType(file, header.Size, "palsu.pdf"); err == nil {
				t.Fatal("ZIP disamarkan PDF diterima")
			}
		})
	}
	header := documentTestFile(t, "uji.txt", []byte("Tentang dokumen"))
	header.Size = MaxDocumentBytes + 1
	if _, err := CreateDocument("D-001", "Uji", header, "uji"); err == nil {
		t.Fatal("ukuran berlebih diterima")
	}
}
