package servicio

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// esSitio indica una petición de navegación o de archivos de la Web UI, que no exige token.
func (s *Servicio) esSitio(r *http.Request) bool {
	if s.opc.Web == nil || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
		return false
	}
	for _, prefijo := range []string{"/api/", "/openai/", "/anthropic/"} {
		if strings.HasPrefix(r.URL.Path, prefijo) {
			return false
		}
	}
	return true
}

// sitio sirve los archivos compilados de la Web UI y, para cualquier otra ruta de navegación, index.html.
func (s *Servicio) sitio(w http.ResponseWriter, r *http.Request) {
	ruta := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if ruta == "" {
		ruta = "index.html"
	}
	if info, err := fs.Stat(s.opc.Web, ruta); err != nil || info.IsDir() {
		ruta = "index.html"
	}
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
	// connect-src admite otros puertos de este equipo para seguir al servicio si un reinicio le cambia el puerto.
	h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; "+
		"connect-src 'self' http://127.0.0.1:* http://localhost:*; frame-ancestors 'none'")
	if strings.HasPrefix(ruta, "assets/") {
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		h.Set("Cache-Control", "no-cache")
	}
	http.ServeFileFS(w, r, s.opc.Web, ruta)
}
