package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Local struct {
	root    string
	tempDir string
}

type localTemp struct {
	*os.File
	path string
}

func NewLocal(root string) (*Local, error) {
	absolute, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil || strings.TrimSpace(root) == "" {
		return nil, errors.New("file storage root is invalid")
	}
	local := &Local{root: absolute, tempDir: filepath.Join(absolute, "temp")}
	for _, directory := range []string{absolute, local.tempDir, filepath.Join(absolute, "objects")} {
		if err := os.MkdirAll(directory, 0o750); err != nil {
			return nil, fmt.Errorf("initialize file storage: %w", err)
		}
	}
	entries, err := os.ReadDir(local.tempDir)
	if err != nil {
		return nil, fmt.Errorf("inspect upload temp directory: %w", err)
	}
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasPrefix(entry.Name(), "upload-") && strings.HasSuffix(entry.Name(), ".part") {
			if err := os.Remove(filepath.Join(local.tempDir, entry.Name())); err != nil {
				return nil, fmt.Errorf("clean stale upload temp: %w", err)
			}
		}
	}
	probe, err := os.CreateTemp(local.tempDir, ".write-probe-")
	if err != nil {
		return nil, fmt.Errorf("file storage is not writable: %w", err)
	}
	probeName := probe.Name()
	closeErr := probe.Close()
	removeErr := os.Remove(probeName)
	if closeErr != nil || removeErr != nil {
		return nil, errors.New("file storage write probe cleanup failed")
	}
	return local, nil
}

func (local *Local) CreateTemp(ctx context.Context) (TempObject, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := os.CreateTemp(local.tempDir, "upload-*.part")
	if err != nil {
		return nil, fmt.Errorf("create upload temp: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return nil, fmt.Errorf("secure upload temp: %w", err)
	}
	return &localTemp{File: file, path: file.Name()}, nil
}

func (local *Local) Commit(ctx context.Context, object TempObject, relative string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	temp, ok := object.(*localTemp)
	if !ok {
		return false, errors.New("invalid local temp object")
	}
	final, err := local.resolve(relative)
	if err != nil {
		return false, err
	}
	if err := temp.File.Sync(); err != nil {
		return false, fmt.Errorf("sync upload temp: %w", err)
	}
	if err := temp.Close(); err != nil {
		return false, fmt.Errorf("close upload temp: %w", err)
	}
	if _, err := os.Stat(final); err == nil {
		_ = os.Remove(temp.path)
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("inspect stored object: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(final), 0o750); err != nil {
		return false, fmt.Errorf("create object directory: %w", err)
	}
	if err := os.Rename(temp.path, final); err != nil {
		if _, statErr := os.Stat(final); statErr == nil {
			_ = os.Remove(temp.path)
			return false, nil
		}
		return false, fmt.Errorf("commit stored object: %w", err)
	}
	if err := os.Chmod(final, 0o640); err != nil {
		_ = os.Remove(final)
		return false, fmt.Errorf("secure stored object: %w", err)
	}
	return true, nil
}

func (local *Local) Abort(object TempObject) error {
	temp, ok := object.(*localTemp)
	if !ok {
		return nil
	}
	_ = temp.Close()
	if err := os.Remove(temp.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove upload temp: %w", err)
	}
	return nil
}

func (local *Local) Open(ctx context.Context, relative string) (Object, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := local.resolve(relative)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open stored object: %w", err)
	}
	return file, nil
}

func (local *Local) Delete(ctx context.Context, relative string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := local.resolve(relative)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete stored object: %w", err)
	}
	return nil
}

func (local *Local) Exists(ctx context.Context, relative string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	path, err := local.resolve(relative)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("inspect stored object: %w", err)
}

func (local *Local) resolve(relative string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(relative))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("invalid storage object path")
	}
	path := filepath.Join(local.root, clean)
	rel, err := filepath.Rel(local.root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("storage object path escapes root")
	}
	return path, nil
}
