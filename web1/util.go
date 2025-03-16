package web1

import (
	"log/slog"
	"net/http"
)

func ListenerLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("Got HTTP request", "RemoteAddress", r.RemoteAddr, "Method", r.Method, "Url", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
