package web

import "embed"

// StaticFS 内嵌控制台静态资源
//
//go:embed static/*
var StaticFS embed.FS
