package refs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type RefManager struct {
	gitDir string
}

func NewRefManager(gitDir string) *RefManager {
	return &RefManager{gitDir: gitDir}
}

func (rm *RefManager) GetHEAD() (string, error) {
	headPath := filepath.Join(rm.gitDir, "HEAD")
	content, err := os.ReadFile(headPath)
	if err != nil {
		return "", fmt.Errorf("failed to read HEAD: %w", err)
	}

	headContent := strings.TrimSpace(string(content))

	// Check if HEAD points to a ref
	if strings.HasPrefix(headContent, "ref: ") {
		refPath := strings.TrimPrefix(headContent, "ref: ")
		return rm.GetRef(refPath)
	}

	// HEAD contains a direct hash
	return headContent, nil
}

func (rm *RefManager) GetRef(refPath string) (string, error) {
	fullPath := filepath.Join(rm.gitDir, refPath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // Ref doesn't exist yet
		}
		return "", fmt.Errorf("failed to read ref %s: %w", refPath, err)
	}

	return strings.TrimSpace(string(content)), nil
}

func (rm *RefManager) SetRef(refPath, hash string) error {
	fullPath := filepath.Join(rm.gitDir, refPath)

	// Create directory if it doesn't exist
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create ref directory: %w", err)
	}

	if err := os.WriteFile(fullPath, []byte(hash+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write ref: %w", err)
	}

	return nil
}

func (rm *RefManager) GetCurrentBranch() (string, error) {
	headPath := filepath.Join(rm.gitDir, "HEAD")
	content, err := os.ReadFile(headPath)
	if err != nil {
		return "", fmt.Errorf("failed to read HEAD: %w", err)
	}

	headContent := strings.TrimSpace(string(content))

	if strings.HasPrefix(headContent, "ref: refs/heads/") {
		return strings.TrimPrefix(headContent, "ref: refs/heads/"), nil
	}

	return "", fmt.Errorf("HEAD is detached")
}

func (rm *RefManager) UpdateCurrentBranch(hash string) error {
	branch, err := rm.GetCurrentBranch()
	if err != nil {
		return err
	}

	refPath := fmt.Sprintf("refs/heads/%s", branch)
	return rm.SetRef(refPath, hash)
}
