package main

import "embed"

//go:embed all:dist
var webDist embed.FS
