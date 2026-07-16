package utils

import (
	"path/filepath"
	"slices"
	"strings"
)

func IsImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	supportedImageExts := []string{
		".jpg", ".jpeg", ".png", ".webp",
	}

	return slices.Contains(supportedImageExts, ext)
}

func IsWebsiteImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	supportedImageExts := []string{
		".jpg", ".jpeg", ".png", ".webp", ".gif", ".svg", ".ico",
	}

	return slices.Contains(supportedImageExts, ext)
}
