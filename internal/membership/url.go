package membership

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// URL return's string represents a member URL.
// The general form represented is:
//
//	{id}={addr}
//
func URL(id uint64, addr string) string {
	return fmt.Sprintf("%d=%s", id, addr)
}

// ParseURL parses a raw url into a member URL structure.
func ParseURL(raw string) (uint64, string, error) {
	idx := strings.Index(raw, "=")
	if idx < 0 {
		return 0, "", errors.New("raft: invalid member url")
	}

	id, err := strconv.ParseUint(raw[:idx], 10, 64)
	if err != nil {
		return 0, "", err
	}

	if id == 0 {
		return 0, "", errors.New("raft: id can't be zero")
	}

	return id, raw[idx+1:], nil
}
