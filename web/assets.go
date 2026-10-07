package web

import "embed"

//go:embed index.html app.js request.js markdown.js transcript.js favicon.svg style.css workflow-model.js connectors.js workflow-panel.js workflows.js workflow-runs.js workflows.css
var Files embed.FS
