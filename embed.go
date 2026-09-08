package web

import "embed"

// DistFS is the Vite production build. Run `npm run build` before `go build`.
//
//go:embed all:dist
var DistFS embed.FS
