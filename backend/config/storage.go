package config

import (
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var UploadsDir = "./uploads"

func init() {
	if err := os.MkdirAll(UploadsDir, os.ModePerm); err != nil {
		log.Printf("[Storage] Warning: Failed to create uploads directory: %v", err)
	}
}

// SanitizeFilename cleans the filename to prevent directory traversal and special character issues.
func SanitizeFilename(name string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	clean := reg.ReplaceAllString(filepath.Base(name), "_")
	return fmt.Sprintf("%d-%s", time.Now().UnixMilli(), clean)
}

// SaveUploadedFile saves an uploaded multipart file to the local uploads directory.
// Returns the public relative URL and file size.
func SaveUploadedFile(fileHeader *multipart.FileHeader) (url string, size int64, err error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", 0, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer file.Close()

	filename := SanitizeFilename(fileHeader.Filename)
	destPath := filepath.Join(UploadsDir, filename)

	dest, err := os.Create(destPath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dest.Close()

	written, err := io.Copy(dest, file)
	if err != nil {
		return "", 0, fmt.Errorf("failed to write file: %w", err)
	}

	// URL format matching existing frontend expectations: /uploads/<filename>
	fileURL := "/uploads/" + filename
	return fileURL, written, nil
}

// DeleteFile removes a file from the uploads directory.
func DeleteFile(fileURL string) error {
	if !strings.HasPrefix(fileURL, "/uploads/") {
		return nil
	}
	relPath := strings.TrimPrefix(fileURL, "/uploads/")
	fullPath := filepath.Join(UploadsDir, filepath.Clean(relPath))
	if _, err := os.Stat(fullPath); err == nil {
		return os.Remove(fullPath)
	}
	return nil
}

// ServeFileInline serves a file (especially PDFs) with inline Content-Disposition so browsers view it natively.
func ServeFileInline(w http.ResponseWriter, r *http.Request, filePath string, originalName string) {
	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		http.Error(w, "Could not read file", http.StatusInternalServerError)
		return
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	contentType := "application/octet-stream"
	switch ext {
	case ".pdf":
		contentType = "application/pdf"
	case ".png":
		contentType = "image/png"
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".webp":
		contentType = "image/webp"
	case ".gif":
		contentType = "image/gif"
	case ".svg":
		contentType = "image/svg+xml"
	case ".txt":
		contentType = "text/plain; charset=utf-8"
	}

	if originalName == "" {
		originalName = filepath.Base(filePath)
	}

	w.Header().Set("Content-Type", contentType)
	// 'inline' ensures the browser renders PDFs and images directly inside iframe or tab instead of forcing download
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, originalName))
	w.Header().Set("Accept-Ranges", "bytes")

	http.ServeContent(w, r, originalName, stat.ModTime(), file)
}
