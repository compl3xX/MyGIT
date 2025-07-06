package commands

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/binary"
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

// CreatePackFile creates a simple pack file without delta compression

// CreatePackFile creates a simple pack file without delta compression
func (gp *GitPush) CreatePackFile(objectHashes []string, objStore *objects.ObjectStore) ([]byte, error) {
	var buf bytes.Buffer

	// Write pack header: "PACK" + version (4 bytes) + number of objects (4 bytes)
	buf.WriteString("PACK")
	buf.Write([]byte{0x00, 0x00, 0x00, 0x02}) // Version 2

	numObjects := uint32(len(objectHashes))
	if err := binary.Write(&buf, binary.BigEndian, numObjects); err != nil {
		return nil, fmt.Errorf("failed to write object count: %w", err)
	}

	// Write all objects
	for _, hash := range objectHashes {
		obj, err := objStore.ReadObject(hash)
		if err != nil {
			return nil, fmt.Errorf("failed to read object %s: %w", hash, err)
		}

		var objHeader string
		switch obj.Type {
		case objects.CommitType:
			objHeader = fmt.Sprintf("commit %d\u0000", len(obj.Content))
		case objects.TreeType:
			objHeader = fmt.Sprintf("tree %d\u0000", len(obj.Content))
		case objects.BlobType:
			objHeader = fmt.Sprintf("blob %d\u0000", len(obj.Content))
		default:
			return nil, fmt.Errorf("unknown object type: %s", obj.Type)
		}

		fullContent := append([]byte(objHeader), obj.Content...)
		err = writeObjectToPack(&buf, fullContent, gp.getObjectTypeNumber(obj.Type))
		if err != nil {
			return nil, fmt.Errorf("failed to write object %s to pack: %w", hash, err)
		}
	}

	// Append SHA-1 checksum of the entire pack (excluding the checksum itself)
	checksum := sha1.Sum(buf.Bytes())
	buf.Write(checksum[:])

	return buf.Bytes(), nil
}
func writeObjectToPack(w io.Writer, content []byte, objType int) error {
	size := len(content)

	// Encode type and size as variable-length header
	var header []byte
	firstByte := byte((objType << 4) | (size & 0x0f))
	size >>= 4

	if size > 0 {
		firstByte |= 0x80 // continuation bit
	}
	header = append(header, firstByte)

	for size > 0 {
		b := byte(size & 0x7f)
		size >>= 7
		if size > 0 {
			b |= 0x80
		}
		header = append(header, b)
	}

	if _, err := w.Write(header); err != nil {
		return fmt.Errorf("failed to write object header: %w", err)
	}

	// Zlib compress the object content
	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	if _, err := zw.Write(content); err != nil {
		return fmt.Errorf("zlib write error: %w", err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("zlib close error: %w", err)
	}

	if _, err := w.Write(compressed.Bytes()); err != nil {
		return fmt.Errorf("failed to write compressed object: %w", err)
	}

	return nil
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

	// Read and parse the smart HTTP response
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Parse the packet-line format response
	var serverRefs = make(map[string]string)
	reader := bytes.NewReader(respData)
	bufReader := bufio.NewReader(reader)

	// Skip the service advertisement line
	firstLine, err := readPktLine(bufReader)
	if err != nil {
		return fmt.Errorf("failed to read service line: %w", err)
	}
	if !strings.Contains(firstLine, "git-receive-pack") {
		return fmt.Errorf("invalid service advertisement: %s", firstLine)
	}

	// Read capabilities line and refs
	for {
		line, err := readPktLine(bufReader)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read ref line: %w", err)
		}

		if line == "" {
			break
		}

		// Parse ref line: "hash refname\0capabilities" or "hash refname"
		if strings.Contains(line, "\000") {
			parts := strings.SplitN(line, "\000", 2)
			refLine := parts[0]
			refParts := strings.SplitN(refLine, " ", 2)
			if len(refParts) == 2 {
				serverRefs[refParts[1]] = refParts[0]
			}
		} else {
			refParts := strings.SplitN(line, " ", 2)
			if len(refParts) == 2 {
				serverRefs[refParts[1]] = refParts[0]
			}
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

	// Create pack file
	packData, err := gp.CreatePackFile(objectHashes, objStore)
	if err != nil {
		return fmt.Errorf("failed to create pack file: %w", err)
	}

	fmt.Printf("Pack file size: %d bytes\n", len(packData))

	os.WriteFile("mytest.pack", packData, 0644)

	// Prepare push request according to Git HTTP protocol
	pushURL := fmt.Sprintf("%s/%s/git-receive-pack", baseURL, repoPath)

	var requestBody bytes.Buffer

	// Write update command
	capabilities := "report-status side-band-64k agent=mygit/0.1"
	updateCmd := fmt.Sprintf("%s %s %s\000%s", remoteCommit, localCommit, remoteBranchRef, capabilities)

	err = writePktLine(&requestBody, updateCmd)
	if err != nil {
		return fmt.Errorf("failed to write update command: %w", err)
	}

	// Write flush packet to end the command section
	err = writePktLine(&requestBody, "")
	if err != nil {
		return fmt.Errorf("failed to write flush packet: %w", err)
	}

	// Append pack file directly (no packet line wrapper for pack data)
	requestBody.Write(packData)

	// Send push request
	pushReq, err := http.NewRequest("POST", pushURL, &requestBody)
	if err != nil {
		return fmt.Errorf("failed to create push request: %w", err)
	}

	pushReq.Header.Set("Content-Type", "application/x-git-receive-pack-request")
	pushReq.Header.Set("Accept", "application/x-git-receive-pack-result")
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
	success := false

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

		if strings.HasPrefix(line, "ok ") {
			success = true
		} else if strings.HasPrefix(line, "ng ") {
			return fmt.Errorf("push rejected: %s", line)
		}
	}

	if success {
		fmt.Println("Push completed successfully!")
	} else {
		fmt.Println("Push completed (no explicit success confirmation)")
	}

	return nil
}
