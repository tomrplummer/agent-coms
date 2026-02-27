package correlation

import (
	"crypto/rand"
	"encoding/base32"
	"regexp"
	"strings"
)

var ridRegex = regexp.MustCompile(`(?i)\[rid:([a-z0-9_-]{3,64})\]`)

func GenerateRID() (string, error) {
	buf := make([]byte, 5)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)
	return strings.ToLower(encoded), nil
}

func NormalizeRID(rid string) string {
	return strings.ToLower(strings.TrimSpace(rid))
}

func IsValidRID(rid string) bool {
	rid = NormalizeRID(rid)
	if len(rid) < 3 || len(rid) > 64 {
		return false
	}
	for _, ch := range rid {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' {
			continue
		}
		return false
	}
	return true
}

func ExtractRID(text string) (string, bool) {
	m := ridRegex.FindStringSubmatch(text)
	if len(m) != 2 {
		return "", false
	}
	return strings.ToLower(m[1]), true
}

func TextContainsRID(text, rid string) bool {
	rid = NormalizeRID(rid)
	if !IsValidRID(rid) {
		return false
	}
	for _, match := range ridRegex.FindAllStringSubmatch(text, -1) {
		if len(match) == 2 && strings.EqualFold(match[1], rid) {
			return true
		}
	}
	return false
}
