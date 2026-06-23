package miser

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
)

var (
	numberRE = regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)
	uuidRE   = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	emailRE  = regexp.MustCompile(`\b[\w.+-]+@[\w.-]+\.[a-zA-Z]{2,}\b`)
	spaceRE  = regexp.MustCompile(`\s+`)
)

func NormalizePrompt(prompt string) string {
	value := strings.ToLower(prompt)
	value = emailRE.ReplaceAllString(value, "<email>")
	value = uuidRE.ReplaceAllString(value, "<uuid>")
	value = numberRE.ReplaceAllString(value, "<num>")
	value = spaceRE.ReplaceAllString(value, " ")
	return strings.TrimSpace(value)
}

func FingerprintPrompt(prompt string) string {
	sum := sha256.Sum256([]byte(NormalizePrompt(prompt)))
	return fmt.Sprintf("%x", sum)[:16]
}
