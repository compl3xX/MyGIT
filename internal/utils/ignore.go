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

	path = filepath.ToSlash(path) // Normalize path to use forward slashes.

	for _, pattern := range i.patterns {
		// Handle directory patterns (e.g., "/logs/" or "node_modules/").
		if strings.HasSuffix(pattern, "/") {
			pattern = strings.TrimSuffix(pattern, "/") // Remove trailing slash.
			// If the pattern starts with a slash, it should match from the root.
			if strings.HasPrefix(pattern, "/") {
				pattern = strings.TrimPrefix(pattern, "/")
				if path == pattern || strings.HasPrefix(path, pattern+"/") {
					return true
				}
			} else {
				// Otherwise, it can match any directory with that name.
				if path == pattern || strings.HasPrefix(path, pattern+"/") || strings.Contains(path, "/"+pattern+"/") {
					return true
				}
			}
			continue // Skip other checks for this pattern.
		}

		// Use filepath.Match for glob pattern matching on the base name.
		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err == nil && matched {
			return true
		}

		// Also check against the full path for patterns like `build/*`.
		matched, err = filepath.Match(pattern, path)
		if err == nil && matched {
			return true
		}
	}

	return false
}
