package utils

import (
	"os"
	"path/filepath"
	"strings"
)

func IsTextFile(filename string) bool {
	// Simple heuristic - check file extension
	ext := strings.ToLower(filepath.Ext(filename))
	textExts := []string{".txt", ".md", ".go", ".py", ".js", ".html", ".css", ".json", ".xml", ".yml", ".yaml"}

	for _, textExt := range textExts {
		if ext == textExt {
			return true
		}
	}
	return false
}

func RelativePath(basePath, targetPath string) (string, error) {
	return filepath.Rel(basePath, targetPath)
}

func PathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
