package message

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/vutran1710/dating-dev/internal/limits"
)

// MaxMessageContentSize is the maximum allowed size for issue body / comment content.
const MaxMessageContentSize = limits.MaxMessageContent

// ValidateContentSize returns an error if content exceeds MaxMessageContentSize.
func ValidateContentSize(content string) error {
	if len(content) > MaxMessageContentSize {
		return fmt.Errorf("content too large: %d bytes (max %d)", len(content), MaxMessageContentSize)
	}
	return nil
}

var markerRe = regexp.MustCompile(`<!-- openpool:([a-z-]+) -->`)

// Format creates a structured message block:
//
//	<!-- openpool:{blockType} -->
//	```
//	{content}
//	```
func Format(blockType, content string) string {
	return fmt.Sprintf("<!-- openpool:%s -->\n```\n%s\n```", blockType, content)
}

// Parse extracts block type and content from a structured message.
// The message may contain surrounding text — only the first openpool block is extracted.
func Parse(raw string) (blockType, content string, err error) {
	match := markerRe.FindStringSubmatchIndex(raw)
	if match == nil {
		return "", "", fmt.Errorf("no openpool marker found")
	}

	blockType = raw[match[2]:match[3]]

	rest := raw[match[1]:]
	fenceStart := strings.Index(rest, "```\n")
	if fenceStart < 0 {
		return "", "", fmt.Errorf("no fenced block after marker")
	}
	afterFence := rest[fenceStart+4:]
	fenceEnd := strings.Index(afterFence, "\n```")
	if fenceEnd < 0 {
		fenceEnd = strings.Index(afterFence, "```")
		if fenceEnd < 0 {
			return "", "", fmt.Errorf("unclosed fenced block")
		}
	}

	content = afterFence[:fenceEnd]
	return blockType, content, nil
}
