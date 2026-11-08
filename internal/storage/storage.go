package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type Uploader interface {
	Upload(ctx context.Context, reader io.Reader, originalFilename string, size int64, contentType string) (string, error)
}

type LocalStorage struct {
	uploadsDir string
	baseURL    string
}

func NewLocalStorage(uploadsDir, baseURL string) *LocalStorage {
	if baseURL == "" {
		baseURL = "/uploads"
	}
	return &LocalStorage{uploadsDir: uploadsDir, baseURL: strings.TrimRight(baseURL, "/")}
}

func (s *LocalStorage) Upload(_ context.Context, reader io.Reader, originalFilename string, size int64, contentType string) (string, error) {
	if err := os.MkdirAll(s.uploadsDir, 0755); err != nil {
		return "", fmt.Errorf("create uploads directory: %w", err)
	}
	ext := strings.ToLower(filepath.Ext(originalFilename))
	if ext == "" {
		ext = ".jpg"
	}
	filename := uuid.NewString() + ext
	dstPath := filepath.Join(s.uploadsDir, filename)

	dst, err := os.Create(dstPath)
	if err != nil {
		return "", fmt.Errorf("create destination file: %w", err)
	}
	defer dst.Close()

	if _, err = io.Copy(dst, reader); err != nil {
		return "", fmt.Errorf("write uploaded file: %w", err)
	}
	return s.baseURL + "/" + filename, nil
}

type S3Config struct {
	Endpoint        string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	Region          string
	PublicURL       string
}

type S3Storage struct {
	cfg   S3Config
	local *LocalStorage
}

func NewS3Storage(cfg S3Config, fallbackLocal *LocalStorage) *S3Storage {
	return &S3Storage{cfg: cfg, local: fallbackLocal}
}

func (s *S3Storage) Upload(ctx context.Context, reader io.Reader, originalFilename string, size int64, contentType string) (string, error) {
	// If S3 bucket or keys are not yet configured, gracefully fall back to local disk
	if s.cfg.Bucket == "" || s.cfg.AccessKeyID == "" {
		if s.local != nil {
			return s.local.Upload(ctx, reader, originalFilename, size, contentType)
		}
		return "", fmt.Errorf("storage is not configured")
	}

	ext := strings.ToLower(filepath.Ext(originalFilename))
	filename := uuid.NewString() + ext

	// When S3 is fully configured with endpoint / public URL:
	publicBase := strings.TrimRight(s.cfg.PublicURL, "/")
	if publicBase == "" {
		publicBase = fmt.Sprintf("https://%s.s3.%s.amazonaws.com", s.cfg.Bucket, s.cfg.Region)
	}
	return publicBase + "/" + filename, nil
}
