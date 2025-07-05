package objects

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ObjectType string

const (
	BlobType   ObjectType = "blob"
	TreeType   ObjectType = "tree"
	CommitType ObjectType = "commit"
)

type Object struct {
	Type    ObjectType
	Size    int
	Content []byte
	Hash    string
}

type ObjectStore struct {
	objectsDir string
}

func NewObjectStore(gitDir string) *ObjectStore {
	return &ObjectStore{objectsDir: filepath.Join(gitDir, "objects")}
}

func (o *ObjectStore) HashObject(content []byte, objType ObjectType) string {
	header := fmt.Sprintf("%s %d\x00", objType, len(content))
	fullContent := append([]byte(header), content...)
	hash := sha1.Sum(fullContent)
	return hex.EncodeToString(hash[:])
}

func (o *ObjectStore) WriteObject(content []byte, objectType ObjectType) (string, error) {
	hash := o.HashObject(content, objectType)

	//create directory for objects(first 2 chars of hash)
	objDir := filepath.Join(o.objectsDir, hash[:2])
	if err := os.MkdirAll(objDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %v", objDir, err)
	}

	// object file path (remaining chars of hash)
	objPath := filepath.Join(objDir, hash[2:])

	if _, err := os.Stat(objPath); err == nil {
		return hash, nil
	}

	header := fmt.Sprintf("%s %d\x00", objectType, len(content))
	fullContent := append([]byte(header), content...)

	//compress content

	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(fullContent); err != nil {
		return "", fmt.Errorf("failed to compress object: %w", err)
	}

	if err := w.Close(); err != nil {
		return "", fmt.Errorf("failed to close zlib writer: %w", err)
	}

	//write the compress content to the file
	if err := os.WriteFile(objPath, buf.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("failed to write object: %w", err)
	}

	return hash, nil
}

func (o *ObjectStore) ReadObject(hash string) (*Object, error) {
	if len(hash) < 4 {
		return nil, fmt.Errorf("hash too short")
	}

	objPath := filepath.Join(o.objectsDir, hash[:2], hash[2:])

	// Read Compressed file
	compressedData, err := os.ReadFile(objPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read object: %w", err)
	}

	//Decompress
	r, err := zlib.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return nil, fmt.Errorf("failed to create decompressor: %w", err)
	}

	defer r.Close()

	decompressed, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress object: %w", err)
	}

	//Parsed header
	nullIndex := bytes.IndexByte(decompressed, 0)
	if nullIndex == -1 {
		return nil, fmt.Errorf("invalid object format: no null terminator")
	}

	header := string(decompressed[:nullIndex])
	content := decompressed[nullIndex+1:]

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid object header format")
	}

	objType := ObjectType(parts[0])

	return &Object{
		Type:    objType,
		Size:    len(content),
		Content: content,
		Hash:    hash,
	}, nil

}

// GetObjectsToSend computes the difference between local and remote objects
func (o *ObjectStore) GetObjectsToSend(localCommit string, remoteCommits []string) ([]string, error) {
	// Get all objects reachable from local commit
	localObjects := make(map[string]bool)
	err := o.traverseObjects(localCommit, localObjects)
	if err != nil {
		return nil, fmt.Errorf("failed to traverse local objects: %w", err)
	}

	// Get all objects reachable from remote commits
	remoteObjects := make(map[string]bool)
	for _, remoteCommit := range remoteCommits {
		if remoteCommit != "0000000000000000000000000000000000000000" {
			err := o.traverseObjects(remoteCommit, remoteObjects)
			if err != nil {
				// Remote object might not exist locally, skip
				continue
			}
		}
	}

	// Find objects that are in local but not in remote
	var objectsToSend []string
	for hash := range localObjects {
		if !remoteObjects[hash] {
			objectsToSend = append(objectsToSend, hash)
		}
	}

	// Sort for consistent ordering
	sort.Strings(objectsToSend)
	return objectsToSend, nil
}

// traverseObjects recursively traverses all objects reachable from a commit
func (o *ObjectStore) traverseObjects(hash string, visited map[string]bool) error {
	if visited[hash] {
		return nil
	}
	visited[hash] = true
	obj, err := o.ReadObject(hash)
	if err != nil {
		return err
	}

	switch obj.Type {
	case "commit":
		lines := strings.Split(string(obj.Content), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "tree ") {
				treeHash := strings.TrimPrefix(line, "tree ")
				if err := o.traverseObjects(treeHash, visited); err != nil {
					return err
				}
			} else if strings.HasPrefix(line, "parent ") {
				parentHash := strings.TrimPrefix(line, "parent ")
				if err := o.traverseObjects(parentHash, visited); err != nil {
					return err
				}
			}
		}
	case "tree":
		data := obj.Content
		for len(data) > 0 {
			nullIndex := bytes.IndexByte(data, 0)
			if nullIndex == -1 {
				break
			}

			if len(data) < nullIndex+21 {
				break
			}

			hashBytes := data[nullIndex+1 : nullIndex+21]
			entryHash := hex.EncodeToString(hashBytes)

			if err := o.traverseObjects(entryHash, visited); err != nil {
				return err
			}

			data = data[nullIndex+21:]
		}
	}

	return nil
}
