package utils

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Ignore represents a set of .gitignore patterns.
type Ignore struct {
	patterns []string
}

// NewIgnore creates a new Ignore instance by loading patterns from the .gitignore file
// in the provided repository work directory.
func NewIgnore(workDir string) (*Ignore, error) {
	gitignorePath := filepath.Join(workDir, ".gitignore")

	file, err := os.Open(gitignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			// .gitignore not found, return an empty ignore list.
			return &Ignore{patterns: []string{}}, nil
		}
		return nil, err
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Ignore empty lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &Ignore{patterns: patterns}, nil
}

// IsIgnored checks if a given file path matches any of the loaded .gitignore patterns.
// The path should be relative to the repository root.
func (i *Ignore) IsIgnored(path string) bool {
	// Always ignore the .mygit directory itself.
	if strings.HasPrefix(path, ".mygit") {
		return true
	}

	for _, pattern := range i.patterns {
		// Use filepath.Match for glob pattern matching.
		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err == nil && matched {
			return true
		}
		// Also check against the full path for patterns like `logs/` or `build/*`
		matched, err = filepath.Match(pattern, path)
		if err == nil && matched {
			return true
		}
	}

	return false
}
