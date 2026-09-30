package web

import "embed"

//go:embed index.html app.js request.js style.css
var Files embed.FS
