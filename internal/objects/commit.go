package objects

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

type Commit struct {
	Tree      string
	Parents   []string
	Author    string
	Committer string
	Message   string
	Timestamp time.Time
}

func NewCommit(treeHash, message, author string, parents []string) *Commit {
	now := time.Now()
	return &Commit{
		Tree:      treeHash,
		Parents:   parents,
		Author:    author,
		Committer: author,
		Message:   message,
		Timestamp: now,
	}
}
func (c *Commit) Serialize() []byte {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("tree %s\n", c.Tree))
	for _, parent := range c.Parents {
		buf.WriteString(fmt.Sprintf("parent %s\n", parent))
	}

	// Timestamp
	timestamp := c.Timestamp.Unix()
	_, offset := c.Timestamp.Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	timezone := fmt.Sprintf("%s%02d%02d", sign, offset/3600, (offset%3600)/60)

	buf.WriteString(fmt.Sprintf("author %s %d %s\n", c.Author, timestamp, timezone))
	buf.WriteString(fmt.Sprintf("committer %s %d %s\n", c.Committer, timestamp, timezone))
	buf.WriteString("\n")
	buf.WriteString(c.Message)

	return buf.Bytes()
}

func ParseCommit(content []byte) (*Commit, error) {
	lines := strings.Split(string(content), "\n")
	commit := &Commit{
		Parents: make([]string, 0),
	}

	messageStart := -1
	for i, line := range lines {
		if line == "" {
			messageStart = i + 1
			break
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}

		switch parts[0] {
		case "tree":
			commit.Tree = parts[1]
		case "parent":
			commit.Parents = append(commit.Parents, parts[1])
		case "author":
			commit.Author = parsePersonWithTimestamp(parts[1])
		case "committer":
			commit.Committer = parsePersonWithTimestamp(parts[1])
		}
	}

	if messageStart >= 0 && messageStart < len(lines) {
		commit.Message = strings.Join(lines[messageStart:], "\n")
	}

	return commit, nil
}

func parsePersonWithTimestamp(line string) string {
	// Format: "Name <email> timestamp timezone"
	// Find the closing bracket of the email to handle names with spaces
	lastAngle := strings.LastIndex(line, ">")
	if lastAngle == -1 {
		// Fallback for unexpected format, though this shouldn't happen with valid commits
		parts := strings.Fields(line)
		if len(parts) > 2 {
			return strings.Join(parts[:len(parts)-2], " ")
		}
		return line
	}
	// The name and email part is everything up to and including the closing angle bracket
	return line[:lastAngle+1]
}
