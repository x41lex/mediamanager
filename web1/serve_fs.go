//go:build !portable
// +build !portable

package web1

import (
	"fmt"
	"net/http"
	"os"
)

func serveFileLive(path string, contentType string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(path)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("failed to read data: %v", err)))
			return
		}
		w.Header().Add("Content-Type", contentType)
		w.WriteHeader(200)
		w.Write(data)
	})
}

func serveFileCached(path string, contentType string) http.HandlerFunc {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("web1: serveFileCached: Failed to load required file '%s': %v", path, err))
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", contentType)
		w.WriteHeader(200)
		w.Write(data)
	})
}
