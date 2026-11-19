package httpx

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
)

const errorPageHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
</head>
<body>
<main>
<h1>{{.Title}}</h1>
<p>{{.Message}}</p>
</main>
</body>
</html>
`

var errorPageTemplate = template.Must(template.New("error-page").Parse(errorPageHTML))

// RespondWithErrorPage renders a minimal HTML error page. The page is rendered
// into a buffer before anything is written, so a rendering failure leaves the
// response untouched and the caller can safely fall back to http.Error.
func RespondWithErrorPage(responseWriter http.ResponseWriter, code int, title string, message string) error {
	var body bytes.Buffer
	err := errorPageTemplate.Execute(&body, struct {
		Title   string
		Message string
	}{Title: title, Message: message})
	if err != nil {
		return fmt.Errorf("render error page: %w", err)
	}

	responseWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
	responseWriter.Header().Set("Cache-Control", "no-store")
	responseWriter.WriteHeader(code)
	if _, err := body.WriteTo(responseWriter); err != nil {
		return fmt.Errorf("write error page: %w", err)
	}
	return nil
}
