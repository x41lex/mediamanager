//go:build portable
// +build portable

package web1

import (
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
)

//go:embed frontend/*.html
//go:embed frontend/*.css
//go:embed js/*.js
var webFs embed.FS

// Verify everythings loaded ok
func init() {
	requiredFiles := map[string][]string{
		"frontend": []string{
			"browse.css",
			"browse.html",
			"collection.css",
			"collection.html",
			"file.css",
			"file.html",
			"general.css",
			"home.css",
			"home.html",
			"login.html",
			"redoc-static.html",
		},
		"js": []string{
			"api.js",
			"browse.js",
			"collection.js",
			"file.js",
			"home.js",
		},
	}
	// Verify required files exist
	for k, v := range requiredFiles {
		dir, err := webFs.ReadDir(k)
		if err != nil {
			panic(fmt.Sprintf("WebEmbed: Failed to read required directory '%s': %v", k, err))
		}
		fileNames := make([]string, len(dir))
		for i, f := range dir {
			fileNames[i] = f.Name()
		}
		// Now we look through the required files, they all must exist.
		for _, r := range v {
			if !slices.Contains(fileNames, r) {
				panic(fmt.Sprintf("WebEmbed: Required file '%s/%s' was not found", k, r))
			}
		}
	}
}

func serveFileLive(path string, contentType string) http.HandlerFunc {
	panic(fmt.Sprintf("MediaManager: web1: Live mode cannot be used on a portable install"))
}

func serveFileCached(path string, contentType string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Remove the web1/
		data, err := webFs.ReadFile(path[5:])
		if err != nil {
			slog.Error("Failed to read embeded file", "Path", path, "Error", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("failed to read data: %v", err)))
			return
		}
		w.Header().Add("Content-Type", contentType)
		w.WriteHeader(200)
		w.Write(data)
	})
}
