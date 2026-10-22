package services

import (
	"archive/zip"
	"bytes"
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lib/pq"
)

// This directory is outside /uploads, which is served publicly.
var DocumentsDir = "private/documents"

const MaxDocumentBytes int64 = 20 * 1024 * 1024

var ErrDocumentNumberExists = errors.New("nomor dokumen sudah digunakan")
var documentIDPattern = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)

const documentColumns = `id,document_number,about,file_name,content_type,file_size,uploaded_by,uploaded_at,download_count`

func scanDocument(row interface{ Scan(...any) error }) (models.Document, error) {
	var d models.Document
	err := row.Scan(&d.ID, &d.DocumentNumber, &d.About, &d.FileName, &d.ContentType, &d.FileSize, &d.UploadedBy, &d.UploadedAt, &d.DownloadCount)
	return d, err
}

func ListDocuments(search string, page, limit int) ([]models.Document, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	search = strings.TrimSpace(search)
	// POSITION treats %, _ and backslashes as literal search text.
	where := `WHERE $1='' OR POSITION(lower($1) IN lower(document_number||' '||about||' '||file_name))>0`
	var total int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM documents `+where, search).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := database.DB.Query(`SELECT `+documentColumns+` FROM documents `+where+` ORDER BY uploaded_at DESC,id DESC LIMIT $2 OFFSET $3`, search, limit, (page-1)*limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	docs := []models.Document{}
	for rows.Next() {
		d, err := scanDocument(rows)
		if err != nil {
			return nil, 0, err
		}
		docs = append(docs, d)
	}
	return docs, total, rows.Err()
}

func documentContentType(file multipart.File, size int64, name string) (string, error) {
	buf := make([]byte, 512)
	n, err := file.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		return "", err
	}
	buf = buf[:n]
	detected := http.DetectContentType(buf)
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".pdf":
		if bytes.HasPrefix(buf, []byte("%PDF-")) {
			return "application/pdf", nil
		}
	case ".png":
		if detected == "image/png" {
			return detected, nil
		}
	case ".jpg", ".jpeg":
		if detected == "image/jpeg" {
			return detected, nil
		}
	case ".txt", ".csv":
		if strings.HasPrefix(detected, "text/plain") {
			if ext == ".csv" {
				return "text/csv", nil
			}
			return "text/plain", nil
		}
	case ".doc", ".xls", ".ppt":
		if bytes.HasPrefix(buf, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}) {
			return map[string]string{".doc": "application/msword", ".xls": "application/vnd.ms-excel", ".ppt": "application/vnd.ms-powerpoint"}[ext], nil
		}
	case ".docx", ".xlsx", ".pptx":
		archive, err := zip.NewReader(file, size)
		if err == nil {
			part := map[string]string{".docx": "word/document.xml", ".xlsx": "xl/workbook.xml", ".pptx": "ppt/presentation.xml"}[ext]
			hasTypes, hasPart := false, false
			for _, entry := range archive.File {
				hasTypes = hasTypes || entry.Name == "[Content_Types].xml"
				hasPart = hasPart || entry.Name == part
			}
			if hasTypes && hasPart {
				return "application/vnd.openxmlformats-officedocument." + map[string]string{".docx": "wordprocessingml.document", ".xlsx": "spreadsheetml.sheet", ".pptx": "presentationml.presentation"}[ext], nil
			}
		}
	default:
		return "", Invalid("Format tidak didukung. Gunakan PDF, Word, Excel, PowerPoint, TXT, CSV, JPG, atau PNG.")
	}
	return "", Invalid("Isi berkas tidak sesuai format dokumen.")
}

func CreateDocument(number, about string, header *multipart.FileHeader, actor string) (models.Document, error) {
	var empty models.Document
	number, about = strings.TrimSpace(number), strings.TrimSpace(about)
	if number == "" || !utf8.ValidString(number) || utf8.RuneCountInString(number) > 120 || strings.IndexFunc(number, unicode.IsControl) >= 0 {
		return empty, Invalid("Nomor dokumen wajib diisi, maksimal 120 karakter tanpa karakter kontrol.")
	}
	if about == "" || !utf8.ValidString(about) || utf8.RuneCountInString(about) > 1000 || strings.ContainsRune(about, 0) {
		return empty, Invalid("Tentang wajib diisi, maksimal 1.000 karakter.")
	}
	if header == nil || header.Size <= 0 || header.Size > MaxDocumentBytes {
		return empty, Invalid("Pilih berkas yang tidak kosong, maksimal 20 MB.")
	}
	name := filepath.Base(strings.ReplaceAll(header.Filename, `\`, "/"))
	name = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name))
	if name == "" || name == "." || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 255 {
		return empty, Invalid("Nama berkas tidak valid atau melebihi 255 karakter.")
	}
	source, err := header.Open()
	if err != nil {
		return empty, err
	}
	defer source.Close()
	contentType, err := documentContentType(source, header.Size, name)
	if err != nil {
		return empty, err
	}
	if err := os.MkdirAll(DocumentsDir, 0700); err != nil {
		return empty, err
	}
	id := NewULID()
	path := filepath.Join(DocumentsDir, id)
	dest, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return empty, err
	}
	stored := false
	defer func() {
		dest.Close()
		if !stored {
			os.Remove(path)
		}
	}()
	n, err := io.Copy(dest, io.LimitReader(source, MaxDocumentBytes+1))
	if err != nil {
		return empty, err
	}
	if n != header.Size || n > MaxDocumentBytes {
		return empty, Invalid("Ukuran berkas tidak sesuai atau melebihi 20 MB.")
	}
	if err := dest.Sync(); err != nil {
		return empty, err
	}
	if err := dest.Close(); err != nil {
		return empty, err
	}
	d, err := scanDocument(database.DB.QueryRow(`INSERT INTO documents(id,document_number,about,file_name,content_type,file_size,uploaded_by)
		VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING `+documentColumns, id, number, about, name, contentType, n, actor))
	if err != nil {
		var pgerr *pq.Error
		if errors.As(err, &pgerr) && pgerr.Code == "23505" && pgerr.Constraint == "idx_documents_number" {
			return empty, ErrDocumentNumberExists
		}
		return empty, err
	}
	stored = true
	return d, nil
}

// Count one accepted download, after opening the file. List/HEAD/failed requests
// do not increment it. Network interruption after sending starts is still counted.
func OpenDocumentDownload(id string) (models.Document, *os.File, error) {
	if !documentIDPattern.MatchString(id) {
		return models.Document{}, nil, sql.ErrNoRows
	}
	d, err := scanDocument(database.DB.QueryRow(`SELECT `+documentColumns+` FROM documents WHERE id=$1`, id))
	if err != nil {
		return d, nil, err
	}
	file, err := os.Open(filepath.Join(DocumentsDir, d.ID))
	if err != nil {
		return d, nil, err
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != d.FileSize {
		file.Close()
		return d, nil, errors.New("berkas dokumen tidak lengkap")
	}
	err = database.DB.QueryRow(`UPDATE documents SET download_count=download_count+1 WHERE id=$1 RETURNING download_count`, d.ID).Scan(&d.DownloadCount)
	if err != nil {
		file.Close()
		return d, nil, err
	}
	return d, file, nil
}
