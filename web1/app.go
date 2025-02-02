package web1

import (
	"net/http"
)

const (
	contentTypeJs   string = "text/javascript"
	contentTypeCss  string = "text/css"
	contentTypeHtml string = "text/html"
)

func (a *DbApi1) initStuff(mux *http.ServeMux, serveFunc func(path string, contentType string) http.HandlerFunc) {
	// CSS Stuff
	mux.Handle("/css/general.css", serveFunc("web1/frontend/general.css", contentTypeCss))
	// API Stuff
	mux.Handle("/js/api.js", serveFunc("web1/js/api.js", contentTypeJs))
	// Browse page
	mux.Handle("/css/browse.css", serveFunc("web1/frontend/browse.css", contentTypeCss))
	mux.Handle("/browse", serveFunc("web1/frontend/browse.html", contentTypeHtml))
	mux.Handle("/js/browse.js", serveFunc("web1/js/browse.js", contentTypeJs))
	// File page
	mux.Handle("/file", serveFunc("web1/frontend/file.html", contentTypeHtml))
	mux.Handle("/css/file.css", serveFunc("web1/frontend/file.css", contentTypeCss))
	mux.Handle("/js/file.js", serveFunc("web1/js/file.js", contentTypeJs))
	// Collection page
	mux.Handle("/collection", serveFunc("web1/frontend/collection.html", contentTypeHtml))
	mux.Handle("/css/collection.css", serveFunc("web1/frontend/collection.css", contentTypeCss))
	mux.Handle("/js/collection.js", serveFunc("web1/js/collection.js", contentTypeJs))
	// Home page
	mux.Handle("/css/home.css", serveFunc("web1/frontend/home.css", contentTypeCss))
	mux.Handle("/js/home.js", serveFunc("web1/js/home.js", contentTypeJs))
	homeServe := serveFunc("web1/frontend/home.html", contentTypeHtml)
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		homeServe(w, r)
	}))
}

func (a *DbApi1) InitApp(mux *http.ServeMux, liveMode bool) {
	if liveMode {
		a.initStuff(mux, serveFileLive)
	} else {
		a.initStuff(mux, serveFileCached)
	}
}
