package httpapi

import (
	"io"
	"net/http"
)

func (a *API) serveAsset(w http.ResponseWriter, name, contentType string) {
	file, err := a.assets.Open(name)
	if err != nil {
		http.NotFound(w, nil)
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = io.Copy(w, file)
}

func (a *API) HandleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	a.serveAsset(w, "index.html", "text/html; charset=utf-8")
}

func (a *API) HandleStyles(w http.ResponseWriter, _ *http.Request) {
	a.serveAsset(w, "styles.css", "text/css; charset=utf-8")
}

func (a *API) HandleScript(w http.ResponseWriter, _ *http.Request) {
	a.serveAsset(w, "app.js", "text/javascript; charset=utf-8")
}

func (a *API) HandleHealth(w http.ResponseWriter, _ *http.Request) {
	writeData(w, http.StatusOK, map[string]string{"status": "ok"}, false)
}
