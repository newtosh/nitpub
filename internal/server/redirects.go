package server

import (
	"net/http"
)

func registerLegacyRedirects(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/enroll", legacyEnrollRedirect)
	mux.HandleFunc("GET /admin/edit/{slug}", legacyEditRedirect)
}

func legacyEnrollRedirect(w http.ResponseWriter, r *http.Request) {
	target := "/author/enroll"
	if q := r.URL.RawQuery; q != "" {
		target += "?" + q
	}
	http.Redirect(w, r, target, http.StatusMovedPermanently)
}

func legacyEditRedirect(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	http.Redirect(w, r, "/author/edit/"+slug, http.StatusMovedPermanently)
}
