package contribution

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type FileStorage interface {
	Save(context.Context, string, io.Reader) (string, error)
	Delete(context.Context, string) error
	SignedURL(context.Context, string, time.Duration) (string, error)
}

type LocalFileStorage struct{ root, publicPrefix string }

func NewLocalFileStorage(root, publicPrefix string) *LocalFileStorage {
	return &LocalFileStorage{root: root, publicPrefix: publicPrefix}
}
func (s *LocalFileStorage) Save(ctx context.Context, extension string, source io.Reader) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(s.root, 0o750); err != nil {
		return "", fmt.Errorf("create proof directory: %w", err)
	}
	name := uuid.NewString() + extension
	path := filepath.Join(s.root, name)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return "", fmt.Errorf("create proof file: %w", err)
	}
	_, copyErr := io.Copy(file, source)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("write proof file: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close proof file: %w", closeErr)
	}
	return s.publicPrefix + "/" + name, nil
}
func (s *LocalFileStorage) Delete(_ context.Context, publicURL string) error {
	return os.Remove(filepath.Join(s.root, filepath.Base(publicURL)))
}
func (s *LocalFileStorage) SignedURL(_ context.Context, key string, _ time.Duration) (string, error) {
	return key, nil
}
