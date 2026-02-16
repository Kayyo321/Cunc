package sending

import (
	"fmt"
	"strings"
)

func ParseToSection(to_before string) (to string, cc string, bcc string, err error) {
	// Parse string like: "sully@bb.com cc: xyz@abc.org bl@yuri.gov bcc: my@mother.smile"

	// Find bcc section first (comes last)
	bcc_index := strings.Index(to_before, "bcc:")
	var bcc_str string
	if bcc_index != -1 {
		bcc_str = strings.TrimSpace(to_before[bcc_index+4:])
		to_before = to_before[:bcc_index]
	}

	// Find cc section
	cc_index := strings.Index(to_before, "cc:")
	var cc_str string
	if cc_index != -1 {
		cc_str = strings.TrimSpace(to_before[cc_index+3:])
		to_before = to_before[:cc_index]
	}

	to_str := strings.TrimSpace(to_before)

	// Validate that we have at least a "to" address
	if to_str == "" {
		return "", "", "", fmt.Errorf("no recipient specified")
	}

	return to_str, cc_str, bcc_str, nil
}
