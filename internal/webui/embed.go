package webui

import (
	"embed"
	"io/fs"
)

//go:embed index.html styles.css app.js
var embedded embed.FS

func Assets() fs.FS {
	return embedded
}
