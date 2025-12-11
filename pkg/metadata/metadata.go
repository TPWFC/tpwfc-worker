// Package metadata provides utilities for extracting and validating metadata from documents.
package metadata

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	// TagStart is the start of the metadata block.
	TagStart = "<!-- METADATA_START"
	// TagEnd is the end of the metadata block.
	TagEnd = "METADATA_END -->"
)

// Metadata verification errors.
var (
	ErrNoMetadataBlock = errors.New("no metadata block found")
	ErrNoHashFound     = errors.New("no hash found in metadata")
	ErrHashMismatch    = errors.New("hash mismatch")
)

// Metadata contains the document status information.
type Metadata struct {
	LastModify time.Time
	Version    string
	Hash       string
	Validation bool
}

// metadataRegex matches the entire metadata block including tags.
var metadataRegex = regexp.MustCompile(`(?s)<!--\s*METADATA_START\s*\n(.*?)\n\s*METADATA_END\s*-->`)

// Extract removes the metadata block from content and returns both the metadata and the cleaned content
// The cleaned content is what should be hashed.
func Extract(content string) (*Metadata, string) {
	match := metadataRegex.FindStringSubmatch(content)
	cleanContent := metadataRegex.ReplaceAllString(content, "")
	// Trim trailing newlines from cleaned content for consistent hashing
	cleanContent = strings.TrimRight(cleanContent, "\n")

	if len(match) < 2 {
		return nil, cleanContent
	}

	meta := &Metadata{}

	lines := strings.SplitSeq(match[1], "\n")
	for line := range lines {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "VALIDATION":
			meta.Validation = (strings.EqualFold(val, "TRUE"))
		case "LAST_MODIFY":
			if t, err := time.Parse(time.RFC3339, val); err == nil {
				meta.LastModify = t
			}
		case "HASH":
			meta.Hash = val
		case "VERSION":
			meta.Version = val
		}
	}

	return meta, cleanContent
}

// CalculateHash computes the SHA-256 hash of the content (excluding metadata).
func CalculateHash(content string) string {
	// Ensure we are hashing the clean content
	_, clean := Extract(content)
	hash := sha256.Sum256([]byte(clean))

	return hex.EncodeToString(hash[:])
}

// Sign appends or updates the metadata block with a fresh hash and timestamp.
func Sign(content string, validated bool) string {
	_, clean := Extract(content)

	// Calculate hash of the clean content
	hash := CalculateHash(clean)

	now := time.Now().UTC().Format(time.RFC3339)

	valStr := "FALSE"
	if validated {
		valStr = "TRUE"
	}

	// Construct new block
	newBlock := fmt.Sprintf("\n\n%s\nVALIDATION: %s\nLAST_MODIFY: %s\nHASH: %s\n%s",
		TagStart, valStr, now, hash, TagEnd)

	return clean + newBlock
}

// Verify checks if the content matches the hash in its metadata.
func Verify(content string) (bool, error) {
	meta, clean := Extract(content)
	if meta == nil {
		return false, ErrNoMetadataBlock
	}

	if meta.Hash == "" {
		return false, ErrNoHashFound
	}

	calculated := CalculateHash(clean)
	if calculated != meta.Hash {
		return false, fmt.Errorf("%w: expected %s, got %s", ErrHashMismatch, meta.Hash, calculated)
	}

	return true, nil
}
