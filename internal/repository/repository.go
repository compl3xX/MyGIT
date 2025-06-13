package repository

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	GitDir     = ".mygit"
	ObjectsDir = "objects"
	RefsDir    = "refs"
	HeadsDir   = "refs/heads"
	TagsDir    = "refs/tags"
	IndexFile  = "index"
	HeadFile   = "HEAD"
)

type GitRepository struct {
	WorkDir string
	GitDir  string
}

func NewGitRepository(workDir string) *GitRepository {
	return &GitRepository{
		WorkDir: workDir,
		GitDir:  filepath.Join(workDir, GitDir),
	}
}

func (r *GitRepository) Init() error {
	dirs := []string{
		r.WorkDir,
		filepath.Join(r.GitDir, ObjectsDir),
		filepath.Join(r.GitDir, RefsDir),
		filepath.Join(r.GitDir, HeadsDir),
		filepath.Join(r.GitDir, TagsDir),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create HEAD file pointing to main branch
	headPath := filepath.Join(r.GitDir, HeadFile)
	headContent := "ref: refs/heads/main\n"
	if err := os.WriteFile(headPath, []byte(headContent), 0644); err != nil {
		return fmt.Errorf("Failed to create head file %s: %v", headPath, err)
	}

	// Create empty index file
	indexPath := filepath.Join(r.GitDir, IndexFile)
	if err := os.WriteFile(indexPath, []byte{}, 0644); err != nil {
		return fmt.Errorf("failed to create index file: %w", err)
	}

	fmt.Printf("Initialized empty Git repository in %s\n", r.GitDir)
	return nil
}

func (r *GitRepository) Exists() bool {
	_, err := os.Stat(r.GitDir)
	return !os.IsNotExist(err)
}

func FindRepository(startPath string) (*GitRepository, error) {
	currentPath := startPath
	for {
		repo := NewGitRepository(currentPath)
		if repo.Exists() {
			return repo, nil
		}

		parent := filepath.Dir(currentPath)
		if parent == currentPath {
			return nil, fmt.Errorf("not a git repository")
		}
		currentPath = parent
	}
}
