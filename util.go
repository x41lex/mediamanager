package main

import (
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
)

const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func GetRandomAccount(accountLen, passwordLen int) (username string, password string) {
	user := make([]byte, accountLen)
	pass := make([]byte, passwordLen)
	for i := range user {
		user[i] = chars[rand.Intn(len(chars))]
	}
	for i := range pass {
		pass[i] = chars[rand.Intn(len(chars))]
	}
	return string(user), string(pass)
}

func copyFile(src string, dst string) error {
	copyFrom, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open src file: %v", err)
	}
	defer copyFrom.Close()
	copyTo, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to open dst file: %v", err)
	}
	defer copyTo.Close()
	_, err = io.Copy(copyTo, copyFrom)
	if err != nil {
		return fmt.Errorf("copy failed: %v", err)
	}
	return nil
}

// Converts a number of bytes into a more readable size
// Possible sizes are Bytes, KB, MB, GB and TB, apart from bytes they will all be rounded to 2 decimal points.
func bytesToString(b float64) string {
	if b >= 1e12 {
		return fmt.Sprintf("%.2f TB", b/1e12)
	} else if b >= 1e9 {
		return fmt.Sprintf("%.2f GB", b/1e9)
	} else if b >= 1e6 {
		return fmt.Sprintf("%.2f MB", b/1e6)
	} else if b >= 1e3 {
		return fmt.Sprintf("%.2f KB", b/1e3)
	}
	return fmt.Sprintf("%.0f", b)
}

func isImportableFile(path string, extraExt ...string) bool {
	ext := filepath.Ext(path)
	if len(ext) == 0 || ext[0] != '.' {
		slog.Error("Invalid file, first value was not a dot", "Path", path, "Ext", ext)
	}
	switch ext {
	// Images
	case ".jpg", ".png", ".jpeg", ".gif":
		return true
	// Audio
	case ".mp3", ".flac", ".wav":
		return true
	// Videos
	case ".webm", ".mp4", ".mov", ".m4v":
		return true
	default:
		return slices.Contains(extraExt, ext)
	}
}
