package sending

import (
	"fmt"
	"strings"
)

func ParseToSection(to_before string) (to string, cc string, bcc string, err error) {
	// Parse string like: "sully@bb.com cc: xyz@abc.org bl@yuri.gov bcc: my@mother.smile"

	// Find bcc section first (comes last)
	bccIndex := strings.Index(to_before, "bcc:")
	var bccStr string
	if bccIndex != -1 {
		bccStr = strings.TrimSpace(to_before[bccIndex+4:])
		to_before = to_before[:bccIndex]
	}

	// Find cc section
	ccIndex := strings.Index(to_before, "cc:")
	var ccStr string
	if ccIndex != -1 {
		ccStr = strings.TrimSpace(to_before[ccIndex+3:])
		to_before = to_before[:ccIndex]
	}

	toStr := strings.TrimSpace(to_before)

	// Validate that we have at least a "to" address
	if toStr == "" {
		return "", "", "", fmt.Errorf("no recipient specified")
	}

	return toStr, ccStr, bccStr, nil
}
