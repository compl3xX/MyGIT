package commands

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"mygit/internal/objects"
	"mygit/internal/repository"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// DeltaEntry represents a delta-compressed object
type DeltaEntry struct {
	BaseHash string
	Data     []byte
	Size     int
}

// PackObject represents an object in a pack file
type PackObject struct {
	Hash   string
	Type   objects.ObjectType
	Size   int
	Data   []byte
	Delta  *DeltaEntry
	Offset int64
}

// GitPush handles the push operation with HTTPS protocol
type GitPush struct {
	repoPath string
	remote   string
	branch   string
	force    bool
	username string
	password string
	timeout  time.Duration
	retries  int
}

// PushOptions contains configuration for push operations
type PushOptions struct {
	Force    bool
	Username string
	Password string
	Timeout  time.Duration
	Retries  int
}

// Push handles the push command
func Push(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: mygit push <remote> <branch> [options]")
		fmt.Println("Example: mygit push origin main")
		fmt.Println("Example: mygit push origin main --force --username=user --password=pass")
		fmt.Println("\nSupported protocol: HTTPS only")
		fmt.Println("Options:")
		fmt.Println("  --force: Force push (non-fast-forward)")
		fmt.Println("  --username=<user>: Username for authentication")
		fmt.Println("  --password=<pass>: Password for authentication")
		return
	}

	remote := args[0]
	branch := args[1]

	// Parse options
	opts := &PushOptions{}

	for i := 2; i < len(args); i++ {
		arg := args[i]

		if arg == "--force" {
			opts.Force = true
		} else if strings.HasPrefix(arg, "--username=") {
			opts.Username = strings.TrimPrefix(arg, "--username=")
		} else if strings.HasPrefix(arg, "--password=") {
			opts.Password = strings.TrimPrefix(arg, "--password=")
		}
	}

	// Get current working directory as repo path
	repoPath, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current directory: %v\n", err)
		return
	}

	repo, err := repository.FindRepository(repoPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	objStore := objects.NewObjectStore(repo.GitDir)

	push := NewGitPush(repoPath, remote, branch, opts)

	// Perform the push
	err = push.Push(objStore)
	if err != nil {
		fmt.Printf("Push failed: %v\n", err)
		os.Exit(1)
	}
}

// NewGitPush creates a new GitPush instance with options
func NewGitPush(repoPath, remote, branch string, opts *PushOptions) *GitPush {
	gp := &GitPush{
		repoPath: repoPath,
		remote:   remote,
		branch:   branch,
		timeout:  30 * time.Second,
		retries:  3,
	}

	if opts != nil {
		gp.force = opts.Force
		gp.username = opts.Username
		gp.password = opts.Password
		if opts.Timeout > 0 {
			gp.timeout = opts.Timeout
		}
		if opts.Retries > 0 {
			gp.retries = opts.Retries
		}
	}

	return gp
}

// CreateDelta creates a delta between two objects
func (gp *GitPush) CreateDelta(baseObj, targetObj *objects.Object) []byte {
	baseData := baseObj.Content
	targetData := targetObj.Content

	if len(baseData) == 0 || len(targetData) == 0 {
		return nil
	}

	// Simple delta implementation - in production, use a more sophisticated algorithm
	var delta bytes.Buffer

	// Write base object size
	gp.writeVarInt(&delta, len(baseData))
	// Write target object size
	gp.writeVarInt(&delta, len(targetData))

	// Find common subsequences and encode differences
	i, j := 0, 0
	for i < len(baseData) && j < len(targetData) {
		// Find matching sequences
		matchLen := 0
		for i+matchLen < len(baseData) && j+matchLen < len(targetData) &&
			baseData[i+matchLen] == targetData[j+matchLen] {
			matchLen++
		}

		if matchLen > 4 { // Only worth encoding if match is significant
			// Copy instruction: offset and length from base
			delta.WriteByte(0x80 | 0x10 | 0x01) // Copy instruction with offset and size
			gp.writeVarInt(&delta, i)
			gp.writeVarInt(&delta, matchLen)
			i += matchLen
			j += matchLen
		} else {
			// Insert instruction: copy from target
			insertLen := 1
			for j+insertLen < len(targetData) && insertLen < 127 {
				// Look ahead to see if we should continue inserting
				nextMatchLen := 0
				for i+nextMatchLen < len(baseData) && j+insertLen+nextMatchLen < len(targetData) &&
					baseData[i+nextMatchLen] == targetData[j+insertLen+nextMatchLen] {
					nextMatchLen++
				}
				if nextMatchLen > 4 {
					break
				}
				insertLen++
			}

			delta.WriteByte(byte(insertLen)) // Insert instruction
			delta.Write(targetData[j : j+insertLen])
			j += insertLen
		}
	}

	// Handle remaining data
	if j < len(targetData) {
		remaining := len(targetData) - j
		delta.WriteByte(byte(remaining))
		delta.Write(targetData[j:])
	}

	return delta.Bytes()
}

// writeVarInt writes a variable-length integer
func (gp *GitPush) writeVarInt(w *bytes.Buffer, value int) {
	for value >= 0x80 {
		w.WriteByte(byte(value) | 0x80)
		value >>= 7
	}
	w.WriteByte(byte(value))
}

// CreatePackFileWithDelta creates an optimized pack file with delta compression
func (gp *GitPush) CreatePackFileWithDelta(objectHashes []string, objStore *objects.ObjectStore) ([]byte, error) {
	var buf bytes.Buffer
	var packObjects []*PackObject

	// Load all objects and create pack objects
	objectMap := make(map[string]*objects.Object)
	for _, hash := range objectHashes {
		obj, err := objStore.ReadObject(hash)
		if err != nil {
			return nil, fmt.Errorf("failed to read object %s: %w", hash, err)
		}
		objectMap[hash] = obj

		packObj := &PackObject{
			Hash: hash,
			Type: obj.Type,
			Size: obj.Size,
			Data: obj.Content,
		}
		packObjects = append(packObjects, packObj)
	}

	// Create deltas for similar objects (simple strategy: same type and similar size)
	for i, obj := range packObjects {
		if obj.Type == objects.BlobType || obj.Type == objects.TreeType {
			bestBase := -1
			bestDelta := []byte(nil)
			bestRatio := 0.5 // Only use delta if it saves at least 50%

			for j, base := range packObjects[:i] {
				if base.Type == obj.Type && base.Delta == nil {
					sizeDiff := float64(abs(base.Size-obj.Size)) / float64(max(base.Size, obj.Size))
					if sizeDiff < 0.5 { // Only try delta if sizes are similar
						delta := gp.CreateDelta(objectMap[base.Hash], objectMap[obj.Hash])
						if len(delta) > 0 {
							ratio := float64(len(delta)) / float64(len(obj.Data))
							if ratio < bestRatio {
								bestBase = j
								bestDelta = delta
								bestRatio = ratio
							}
						}
					}
				}
			}

			if bestBase >= 0 {
				obj.Delta = &DeltaEntry{
					BaseHash: packObjects[bestBase].Hash,
					Data:     bestDelta,
					Size:     len(bestDelta),
				}
			}
		}
	}

	// Pack file header
	buf.WriteString("PACK")
	buf.Write([]byte{0, 0, 0, 2}) // version 2

	// Number of objects (4 bytes, big endian)
	numObjects := len(packObjects)
	buf.WriteByte(byte(numObjects >> 24))
	buf.WriteByte(byte(numObjects >> 16))
	buf.WriteByte(byte(numObjects >> 8))
	buf.WriteByte(byte(numObjects))

	// Write objects
	for _, packObj := range packObjects {
		var objType int
		var data []byte
		var size int

		if packObj.Delta != nil {
			objType = 7 // OBJ_REF_DELTA
			data = packObj.Delta.Data
			size = packObj.Delta.Size
		} else {
			objType = gp.getObjectTypeNumber(packObj.Type)
			data = packObj.Data
			size = len(data)
		}

		// Encode type and size
		firstByte := (objType << 4) | (size & 0x0f)
		if size >= 16 {
			firstByte |= 0x80
		}
		buf.WriteByte(byte(firstByte))

		size >>= 4
		for size > 0 {
			b := byte(size & 0x7f)
			size >>= 7
			if size > 0 {
				b |= 0x80
			}
			buf.WriteByte(b)
		}

		// For delta objects, write base reference
		if packObj.Delta != nil {
			// Write base hash (20 bytes)
			baseHash, _ := hex.DecodeString(packObj.Delta.BaseHash)
			buf.Write(baseHash)
		}

		// Compress and write object data
		var compressed bytes.Buffer
		zlibWriter := zlib.NewWriter(&compressed)
		zlibWriter.Write(data)
		zlibWriter.Close()

		buf.Write(compressed.Bytes())
	}

	// Calculate and append checksum
	hash := sha1.Sum(buf.Bytes())
	buf.Write(hash[:])

	return buf.Bytes(), nil
}

// Helper functions
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// getObjectTypeNumber returns the numeric type for git objects
func (gp *GitPush) getObjectTypeNumber(objType objects.ObjectType) int {
	switch objType {
	case objects.CommitType:
		return 1
	case objects.TreeType:
		return 2
	case objects.BlobType:
		return 3
	default:
		return 0
	}
}

// GetCurrentBranchCommit returns the current commit hash for a branch
func (gp *GitPush) GetCurrentBranchCommit() (string, error) {
	branchPath := filepath.Join(gp.repoPath, ".mygit", "refs", "heads", gp.branch)

	data, err := os.ReadFile(branchPath)
	if err != nil {
		return "", fmt.Errorf("branch not found: %s", gp.branch)
	}

	return strings.TrimSpace(string(data)), nil
}

// GetRemoteURL returns the URL for the specified remote
func (gp *GitPush) GetRemoteURL() (string, error) {
	configPath := filepath.Join(gp.repoPath, ".mygit", "config")

	file, err := os.Open(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to open git config: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inRemoteSection := false
	correctRemote := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "[remote ") {
			remoteName := strings.Trim(strings.TrimPrefix(line, "[remote "), "\"]")
			correctRemote = (remoteName == gp.remote)
			inRemoteSection = true
		} else if strings.HasPrefix(line, "[") {
			inRemoteSection = false
			correctRemote = false
		} else if inRemoteSection && correctRemote && strings.HasPrefix(line, "url = ") {
			return strings.TrimPrefix(line, "url = "), nil
		}
	}

	return "", fmt.Errorf("remote not found: %s", gp.remote)
}

// parseHTTPSURL parses HTTPS Git URL and returns host, port, and path
func (gp *GitPush) parseHTTPSURL(gitURL string) (host, port, path string, err error) {
	parsedURL, err := url.Parse(gitURL)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid HTTPS URL: %w", err)
	}

	host = parsedURL.Hostname()
	port = parsedURL.Port()
	if port == "" {
		port = "443"
	}
	path = strings.TrimPrefix(parsedURL.Path, "/")
	if strings.HasSuffix(path, ".git") {
		path = strings.TrimSuffix(path, ".git")
	}

	return host, port, path, nil
}

// writePktLine writes a git packet line
func writePktLine(w io.Writer, data string) error {
	if data == "" {
		_, err := w.Write([]byte("0000"))
		return err
	}

	length := len(data) + 4
	pktLine := fmt.Sprintf("%04x%s", length, data)
	_, err := w.Write([]byte(pktLine))
	return err
}

// readPktLine reads a git packet line
func readPktLine(r *bufio.Reader) (string, error) {
	lengthBytes := make([]byte, 4)
	_, err := io.ReadFull(r, lengthBytes)
	if err != nil {
		return "", err
	}

	lengthStr := string(lengthBytes)
	if lengthStr == "0000" {
		return "", nil
	}

	length, err := strconv.ParseInt(lengthStr, 16, 32)
	if err != nil {
		return "", fmt.Errorf("invalid packet length: %s", lengthStr)
	}

	if length < 4 {
		return "", fmt.Errorf("packet too short: %d", length)
	}

	dataBytes := make([]byte, length-4)
	_, err = io.ReadFull(r, dataBytes)
	if err != nil {
		return "", err
	}

	return string(dataBytes), nil
}

// Push performs the git push operation using HTTPS protocol
func (gp *GitPush) Push(objStore *objects.ObjectStore) error {
	fmt.Printf("Pushing %s to %s/%s...\n", gp.branch, gp.remote, gp.branch)

	// Get current branch commit
	localCommit, err := gp.GetCurrentBranchCommit()
	if err != nil {
		return fmt.Errorf("failed to get local commit: %w", err)
	}

	// Get remote URL
	remoteURL, err := gp.GetRemoteURL()
	if err != nil {
		return fmt.Errorf("failed to get remote URL: %w", err)
	}

	// Parse HTTPS URL
	host, port, repoPath, err := gp.parseHTTPSURL(remoteURL)
	if err != nil {
		return fmt.Errorf("failed to parse remote URL: %w", err)
	}

	fmt.Printf("Using HTTPS protocol to connect to %s:%s...\n", host, port)

	baseURL := fmt.Sprintf("https://%s", host)
	if port != "443" {
		baseURL = fmt.Sprintf("https://%s:%s", host, port)
	}

	// First, discover references
	discoverURL := fmt.Sprintf("%s/%s/info/refs?service=git-receive-pack", baseURL, repoPath)

	client := &http.Client{Timeout: gp.timeout}

	req, err := http.NewRequest("GET", discoverURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if gp.username != "" && gp.password != "" {
		req.SetBasicAuth(gp.username, gp.password)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to discover references: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("reference discovery failed: %s", resp.Status)
	}

	// Parse references
	scanner := bufio.NewScanner(resp.Body)
	var serverRefs = make(map[string]string)

	// Skip the first line (service advertisement)
	if scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "git-receive-pack") {
			return fmt.Errorf("invalid service advertisement: %s", line)
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 4 {
			continue
		}

		// Remove packet length prefix
		if len(line) >= 4 {
			line = line[4:]
		}

		if line == "" {
			break
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}

		hash := parts[0]
		refAndCaps := parts[1]

		if strings.Contains(refAndCaps, "\000") {
			refCapsParts := strings.SplitN(refAndCaps, "\000", 2)
			refName := refCapsParts[0]
			serverRefs[refName] = hash
		} else {
			serverRefs[refAndCaps] = hash
		}
	}

	// Check remote branch status
	remoteBranchRef := fmt.Sprintf("refs/heads/%s", gp.branch)
	remoteCommit := "0000000000000000000000000000000000000000"
	if hash, exists := serverRefs[remoteBranchRef]; exists {
		remoteCommit = hash
	}

	// Check if push is needed
	if remoteCommit == localCommit {
		fmt.Println("Everything up-to-date")
		return nil
	}

	// Check fast-forward
	if !gp.force && remoteCommit != "0000000000000000000000000000000000000000" {
		fmt.Println("Note: This is a simplified implementation. Fast-forward checking is basic.")
	}

	// Get objects to send
	remoteCommits := []string{remoteCommit}
	objectHashes, err := objStore.GetObjectsToSend(localCommit, remoteCommits)
	if err != nil {
		return fmt.Errorf("failed to compute objects to send: %w", err)
	}

	if len(objectHashes) == 0 {
		fmt.Println("Everything up-to-date")
		return nil
	}

	fmt.Printf("Objects to push: %d\n", len(objectHashes))

	// Create pack file with delta compression
	packData, err := gp.CreatePackFileWithDelta(objectHashes, objStore)
	if err != nil {
		return fmt.Errorf("failed to create pack file: %w", err)
	}

	fmt.Printf("Pack file size: %d bytes\n", len(packData))

	// Prepare push request
	pushURL := fmt.Sprintf("%s/%s/git-receive-pack", baseURL, repoPath)

	var requestBody bytes.Buffer

	// Write update command
	updateCmd := fmt.Sprintf("%s %s %s\000report-status", remoteCommit, localCommit, remoteBranchRef)
	writePktLine(&requestBody, updateCmd)
	writePktLine(&requestBody, "") // Flush packet

	// Append pack file
	requestBody.Write(packData)

	// Send push request
	pushReq, err := http.NewRequest("POST", pushURL, &requestBody)
	if err != nil {
		return fmt.Errorf("failed to create push request: %w", err)
	}

	pushReq.Header.Set("Content-Type", "application/x-git-receive-pack-request")
	if gp.username != "" && gp.password != "" {
		pushReq.SetBasicAuth(gp.username, gp.password)
	}

	pushResp, err := client.Do(pushReq)
	if err != nil {
		return fmt.Errorf("push request failed: %w", err)
	}
	defer pushResp.Body.Close()

	if pushResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(pushResp.Body)
		return fmt.Errorf("push failed: %s - %s", pushResp.Status, string(body))
	}

	// Read response
	respReader := bufio.NewReader(pushResp.Body)
	for {
		line, err := readPktLine(respReader)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read response: %w", err)
		}

		if line == "" {
			break
		}

		fmt.Printf("Server: %s\n", line)

		if strings.HasPrefix(line, "ng ") {
			return fmt.Errorf("push rejected: %s", line)
		}
	}

	fmt.Println("Push completed successfully!")
	return nil
}
