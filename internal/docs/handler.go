package docs

import (
	"embed"
	"net/http"
)

//go:embed openapi.yaml
var files embed.FS

func Spec(w http.ResponseWriter, r *http.Request) {
	data, err := files.ReadFile("openapi.yaml")
	if err != nil {
		http.Error(w, "documentation unavailable", 500)
		return
	}
	w.Header().Set("Content-Type", "application/yaml")
	_, _ = w.Write(data)
}
func UI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html><html><head><title>Social Fund API</title><link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css"></head><body><div id="swagger-ui"></div><script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script><script>SwaggerUIBundle({url:'/swagger/openapi.yaml',dom_id:'#swagger-ui',deepLinking:true,persistAuthorization:true})</script></body></html>`))
}
