package tags

import (
	"encoding/hex"
	"fmt"
	"strconv"
)

// Find returns the first tag with the given key, or nil.
func Find(tags [][]string, key string) []string {
	for _, tag := range tags {
		if len(tag) > 0 && tag[0] == key {
			return tag
		}
	}
	return nil
}

// FindAll returns all tags with the given key.
func FindAll(tags [][]string, key string) [][]string {
	var result [][]string
	for _, tag := range tags {
		if len(tag) > 0 && tag[0] == key {
			result = append(result, tag)
		}
	}
	return result
}

// RequireString extracts a single string value from a tag, returning an error if missing.
func RequireString(tags [][]string, key string) (string, error) {
	tag := Find(tags, key)
	if tag == nil || len(tag) < 2 {
		return "", fmt.Errorf("required tag '%s' not found", key)
	}
	return tag[1], nil
}

// RequireInt64 extracts and parses an int64 value from a tag.
func RequireInt64(tags [][]string, key string) (int64, error) {
	s, err := RequireString(tags, key)
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("tag '%s' invalid int64: %w", key, err)
	}
	return v, nil
}

// OptionalString extracts a string value, returning "" if the tag is absent.
func OptionalString(tags [][]string, key string) string {
	tag := Find(tags, key)
	if tag == nil || len(tag) < 2 {
		return ""
	}
	return tag[1]
}

// OptionalInt64 extracts an int64 value, returning 0 if the tag is absent.
// Returns an error only if the tag exists but cannot be parsed.
func OptionalInt64(tags [][]string, key string) (int64, error) {
	tag := Find(tags, key)
	if tag == nil || len(tag) < 2 {
		return 0, nil
	}
	v, err := strconv.ParseInt(tag[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("tag '%s' invalid int64: %w", key, err)
	}
	return v, nil
}

// Validate32Hex checks that s is a valid 64-char lowercase hex string (32 bytes).
func Validate32Hex(s string) error {
	if len(s) != 64 {
		return fmt.Errorf("expected 64 hex chars, got %d", len(s))
	}
	_, err := hex.DecodeString(s)
	if err != nil {
		return fmt.Errorf("invalid hex: %w", err)
	}
	return nil
}
