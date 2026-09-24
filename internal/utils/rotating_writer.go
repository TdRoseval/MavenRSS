package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

const (
	defaultLogMaxSize  int64 = 100 * 1024 * 1024
	defaultLogMaxFiles       = 5
)

// RotatingFileConfig controls the size and retention policy of a RotatingFile.
// MaxFiles is the number of rotated backups retained in addition to the active
// file (for example, debug.log.1 through debug.log.5).
type RotatingFileConfig struct {
	MaxSize        int64
	MaxFiles       int
	TruncateOnOpen bool
}

// RotatingFile is a thread-safe size-based rotating file writer.
type RotatingFile struct {
	mu       sync.Mutex
	path     string
	maxSize  int64
	maxFiles int
	file     *os.File
	size     int64
}

// NewRotatingFile opens path using config. Existing content is preserved by
// default; callers that historically truncated a log at startup can set
// TruncateOnOpen.
func NewRotatingFile(path string, config RotatingFileConfig) (*RotatingFile, error) {
	if config.MaxSize <= 0 {
		config.MaxSize = defaultLogMaxSize
	}
	if config.MaxFiles < 1 {
		config.MaxFiles = defaultLogMaxFiles
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	flags := os.O_CREATE | os.O_WRONLY
	if config.TruncateOnOpen {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_APPEND
	}
	file, err := os.OpenFile(path, flags, 0666)
	if err != nil {
		return nil, err
	}

	var size int64
	if info, statErr := file.Stat(); statErr == nil {
		size = info.Size()
	}
	return &RotatingFile{
		path:     path,
		maxSize:  config.MaxSize,
		maxFiles: config.MaxFiles,
		file:     file,
		size:     size,
	}, nil
}

// NewRotatingFileFromEnv uses MRRSS_LOG_MAX_SIZE_MB and MRRSS_LOG_MAX_FILES,
// falling back to 100 MiB and 5 rotated files when unset or invalid.
func NewRotatingFileFromEnv(path string, truncateOnOpen bool) (*RotatingFile, error) {
	config := RotatingFileConfig{
		MaxSize:        defaultLogMaxSize,
		MaxFiles:       defaultLogMaxFiles,
		TruncateOnOpen: truncateOnOpen,
	}
	if value, err := strconv.ParseInt(os.Getenv("MRRSS_LOG_MAX_SIZE_MB"), 10, 64); err == nil && value > 0 {
		config.MaxSize = value * 1024 * 1024
	}
	if value, err := strconv.Atoi(os.Getenv("MRRSS_LOG_MAX_FILES")); err == nil && value > 0 {
		config.MaxFiles = value
	}
	return NewRotatingFile(path, config)
}

// Write appends p and rotates before the active file exceeds MaxSize.
func (f *RotatingFile) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.file == nil {
		return 0, os.ErrClosed
	}
	written := 0
	for len(p) > 0 {
		if f.size >= f.maxSize {
			if err := f.rotateLocked(); err != nil {
				return written, err
			}
		}

		remaining := f.maxSize - f.size
		chunkSize := int64(len(p))
		if chunkSize > remaining {
			chunkSize = remaining
		}
		n, err := f.file.Write(p[:chunkSize])
		f.size += int64(n)
		written += n
		p = p[n:]
		if err != nil {
			return written, err
		}
		if n == 0 {
			return written, io.ErrShortWrite
		}
	}
	return written, nil
}

// Sync flushes the active log file to storage.
func (f *RotatingFile) Sync() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.file == nil {
		return os.ErrClosed
	}
	return f.file.Sync()
}

// Close closes the active log file. It is safe to call more than once.
func (f *RotatingFile) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.file == nil {
		return nil
	}
	err := f.file.Close()
	f.file = nil
	return err
}

func (f *RotatingFile) rotateLocked() error {
	if err := f.file.Close(); err != nil {
		return err
	}
	f.file = nil

	for index := f.maxFiles; index >= 1; index-- {
		oldPath := f.path
		if index > 1 {
			oldPath = fmt.Sprintf("%s.%d", f.path, index-1)
		}
		newPath := fmt.Sprintf("%s.%d", f.path, index)
		if err := os.Remove(newPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.Rename(oldPath, newPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	file, err := os.OpenFile(f.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	f.file = file
	f.size = 0
	return nil
}
