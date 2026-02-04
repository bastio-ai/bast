// Package stdin provides utilities for handling piped input
package stdin

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// MaxInputSize is the maximum size of input to process (100KB)
const MaxInputSize = 100 * 1024

// HeadSize is how much to keep from the beginning when truncating
const HeadSize = 40 * 1024

// TailSize is how much to keep from the end when truncating
const TailSize = 40 * 1024

// IsPiped returns true if stdin has piped input
func IsPiped() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) == 0
}

// Read reads all content from stdin up to MaxInputSize
func Read() (string, error) {
	return ReadFrom(os.Stdin)
}

// ReadFrom reads all content from the given reader up to MaxInputSize
func ReadFrom(r io.Reader) (string, error) {
	var sb strings.Builder
	reader := bufio.NewReader(r)
	buf := make([]byte, 4096)
	totalRead := 0

	for totalRead < MaxInputSize {
		n, err := reader.Read(buf)
		if n > 0 {
			// Don't exceed max size
			if totalRead+n > MaxInputSize {
				n = MaxInputSize - totalRead
			}
			sb.Write(buf[:n])
			totalRead += n
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return sb.String(), err
		}
	}

	return sb.String(), nil
}

// Truncate intelligently truncates content to fit within maxSize
// Preserves content from head and tail, inserting a marker in the middle
func Truncate(content string, maxSize int) string {
	if len(content) <= maxSize {
		return content
	}

	// Calculate how much we need to cut
	headSize := HeadSize
	tailSize := TailSize

	// Adjust if maxSize is smaller than our default head+tail
	if maxSize < headSize+tailSize {
		headSize = maxSize / 2
		tailSize = maxSize - headSize
	}

	// Find good break points (newlines) near our cut points
	head := content[:headSize]
	tail := content[len(content)-tailSize:]

	// Try to break at newline for cleaner output
	if idx := strings.LastIndex(head, "\n"); idx > headSize/2 {
		head = content[:idx+1]
	}
	if idx := strings.Index(tail, "\n"); idx > 0 && idx < tailSize/2 {
		tail = content[len(content)-tailSize+idx+1:]
	}

	omitted := len(content) - len(head) - len(tail)
	marker := fmt.Sprintf("\n[... %d bytes omitted ...]\n", omitted)
	return head + marker + tail
}

// TruncateLines truncates content to a maximum number of lines
func TruncateLines(content string, maxLines int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return content
	}

	// Keep first half and last half of lines
	headLines := maxLines / 2
	tailLines := maxLines - headLines

	result := strings.Join(lines[:headLines], "\n")
	result += fmt.Sprintf("\n[... %d lines omitted ...]\n", len(lines)-maxLines)
	result += strings.Join(lines[len(lines)-tailLines:], "\n")

	return result
}
