package handlers

import (
	"io/fs"
	"net/http"
	"strconv"

	"github.com/salandered/wavelen/internal/version"
)

/*
NewStaticHandler serves the embedded UI.

Embedding (embed.FS) to the app binary makes files lose their OS "Modified Time".
http.FileServer relies on that to handle caching.
With zero mod time it refetches app.js and other files on every reload.

Since UI files can only change when a new app binary is compiled,
we can use the app's build version as the ETag.

[http.ServeContent] would read the browser's incoming If-None-Match request,
match it against the build ver, and auto respond with 304 Not Modified.
*/
func NewStaticHandler(files fs.FS) http.Handler {
	fileServer := readOnly(http.FileServerFS(files))
	if version.Get() == version.Dev {
		return fileServer // Assets can be changed in dev environment
	}

	etag := strconv.Quote(version.Get())
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Etag", etag)
		w.Header().Set("Cache-Control", "no-cache") // cache, but revalidate
		fileServer.ServeHTTP(w, req)
	})
}

// Catch all for random requests: sees every request no route claimed, and return 404.
// [http.FileServerFS] is permissive and would've ignored the method and served a file as if it was a GET.
func readOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet && req.Method != http.MethodHead {
			http.NotFound(w, req)
			return
		}
		next.ServeHTTP(w, req)
	})
}
