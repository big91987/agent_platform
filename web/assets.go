package web

import "embed"

//go:embed index.html app.js request.js markdown.js transcript.js favicon.svg style.css
var Files embed.FS
