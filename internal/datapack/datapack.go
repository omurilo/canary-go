// Package datapack handles downloading the OTBM map and other datapack
// assets on startup, mirroring the C++ docker/data/start.sh behavior.
package datapack

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// downloadProgressReader wraps an io.Reader and logs progress at intervals.
type downloadProgressReader struct {
	reader    io.Reader
	total     int64
	downloaded int64
	lastLog   time.Time
	log       *slog.Logger
	label     string
}

func (d *downloadProgressReader) Read(p []byte) (int, error) {
	n, err := d.reader.Read(p)
	d.downloaded += int64(n)
	if d.lastLog.IsZero() || time.Since(d.lastLog) > 5*time.Second {
		if d.total > 0 {
			pct := float64(d.downloaded) / float64(d.total) * 100
			d.log.Info(fmt.Sprintf("%s: %.1f%% (%d/%d MB)", d.label, pct, d.downloaded/1024/1024, d.total/1024/1024))
		} else {
			d.log.Info(fmt.Sprintf("%s: %d MB downloaded", d.label, d.downloaded/1024/1024))
		}
		d.lastLog = time.Now()
	}
	return n, err
}

// EnsureMap checks whether the OTBM map file at mapFilePath exists. If it
// doesn't and enabled is true with a non-empty downloadURL, it downloads the
// map to that location. Returns nil when the file exists or was downloaded
// successfully, or when downloads are disabled/not configured (the caller
// should fall back to the synthetic spawn field).
func EnsureMap(log *slog.Logger, mapFilePath, downloadURL string, enabled bool) error {
	if mapFilePath == "" {
		return nil
	}

	// Already present — nothing to do.
	if _, err := os.Stat(mapFilePath); err == nil {
		return nil
	}

	if !enabled {
		log.Warn("OTBM map not found and auto-download disabled; will fall back to synthetic spawn field",
			"file", mapFilePath)
		return nil
	}

	if downloadURL == "" {
		log.Warn("OTBM map not found and no download URL configured; will fall back to synthetic spawn field",
			"file", mapFilePath)
		return nil
	}

	log.Info("OTBM map not found locally; downloading", "url", downloadURL, "dest", mapFilePath)

	if err := downloadMap(log, downloadURL, mapFilePath); err != nil {
		return fmt.Errorf("download map from %s: %w", downloadURL, err)
	}

	log.Info("OTBM map downloaded successfully", "file", mapFilePath)
	return nil
}

// downloadMap downloads url to destPath with progress logging.
func downloadMap(log *slog.Logger, url, destPath string) error {
	// Ensure the destination directory exists.
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Canary-Go/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	}

	contentLength := resp.ContentLength
	log.Info(fmt.Sprintf("Downloading %s (%.1f MB)", destPath, float64(contentLength)/1024/1024))

	// Download to a temporary file first for atomic rename.
	tmpFile, err := os.CreateTemp(dir, "*.otbm.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // Clean up on error.

	progress := &downloadProgressReader{
		reader: resp.Body,
		total:  contentLength,
		log:    log,
		label:  filepath.Base(destPath),
	}

	written, err := io.Copy(tmpFile, progress)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("download write: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if contentLength > 0 && written != contentLength {
		return fmt.Errorf("incomplete download: wrote %d bytes, expected %d", written, contentLength)
	}

	// Atomic rename: temp file → destination.
	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("rename %s to %s: %w", tmpPath, destPath, err)
	}

	return nil
}
