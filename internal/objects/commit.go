package objects

import (
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

	var lines []string
	lines = append(lines, fmt.Sprintf("tree %s", c.Tree))

	for _, parent := range c.Parents {
		lines = append(lines, fmt.Sprintf("parent %s", parent))
	}

	timestamp := c.Timestamp.Unix()
	timezone := c.Timestamp.Format("-0700")

	lines = append(lines, fmt.Sprintf("author %s %d %s", c.Author, timestamp, timezone))
	lines = append(lines, fmt.Sprintf("committer %s %d %s", c.Committer, timestamp, c.Message))
	lines = append(lines, "")
	lines = append(lines, c.Message)

	return []byte(strings.Join(lines, "\n"))
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
	// For simplicity, just return the name and email part
	parts := strings.Fields(line)
	if len(parts) >= 2 {
		// Find the last two parts (timestamp and timezone) and exclude them
		nameEmailParts := parts[:len(parts)-2]
		return strings.Join(nameEmailParts, " ")
	}
	return line
}
