// Package bss 仅承载模块根级的 embed 资源。
package bss

import "embed"

// MigrationsFS 内嵌全部 SQL migration 文件（goose 使用）
//go:embed all:migrations
var MigrationsFS embed.FS

// WebDistFS 内嵌前端构建产物（交付 = 单二进制；开发期先放占位文件，make build 后覆盖）
//go:embed all:web/dist
var WebDistFS embed.FS
