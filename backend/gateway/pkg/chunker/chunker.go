// Package chunker provides text chunking for outbound messages.
//
// It splits long messages into platform-compatible chunks, preserving
// readability by cutting at paragraph, newline, word, or hard boundaries
// in descending order of preference. Platforms declare their TextFormatMode
// (plain/markdown) and max message length.
package chunker

import (
	"strings"
)

// TextFormatMode describes the output format of the text.
type TextFormatMode string

const (
	TextFormatPlain    TextFormatMode = "plain"
	TextFormatMarkdown TextFormatMode = "markdown"
)

// Chunker splits long text into platform-sized chunks.
type Chunker struct {
	Mode         TextFormatMode
	MaxChunkLen  int
}

// New creates a Chunker with the given mode and maximum chunk length.
func New(mode TextFormatMode, maxLen int) *Chunker {
	if maxLen <= 0 {
		maxLen = 4000
	}
	return &Chunker{Mode: mode, MaxChunkLen: maxLen}
}

// Split splits text into chunks respecting format boundaries.
// Returns a single-element slice if the text fits in one chunk.
func (c *Chunker) Split(text string) []string {
	if len(text) <= c.MaxChunkLen {
		return []string{text}
	}

	switch c.Mode {
	case TextFormatMarkdown:
		return c.splitMarkdown(text)
	default:
		return c.splitPlain(text)
	}
}

// splitPlain splits plain text by: paragraph → newline → word → hard cut.
func (c *Chunker) splitPlain(text string) []string {
	return c.splitWith(text, func(s string) []string {
		return c.splitByParagraph(s)
	})
}

// splitMarkdown splits markdown preserving code blocks and lists.
func (c *Chunker) splitMarkdown(text string) []string {
	return c.splitWith(text, func(s string) []string {
		// For markdown, prefer splitting at double newlines (paragraphs)
		// to avoid breaking code blocks and lists.
		return c.splitByParagraph(s)
	})
}

// splitWith is the recursive chunking driver.
func (c *Chunker) splitWith(text string, splitter func(string) []string) []string {
	if len(text) <= c.MaxChunkLen {
		return []string{text}
	}

	// Try paragraph split first
	paragraphs := splitter(text)
	if len(paragraphs) > 1 {
		return c.mergeChunks(paragraphs)
	}

	// Try newline split
	lines := strings.Split(text, "\n")
	if len(lines) > 1 {
		return c.mergeChunks(lines)
	}

	// Try word split (space)
	if strings.Contains(text, " ") {
		return c.splitBySpace(text)
	}

	// Hard cut
	return c.hardSplit(text)
}

// mergeChunks merges segments into chunks that fit within MaxChunkLen.
func (c *Chunker) mergeChunks(segments []string) []string {
	var chunks []string
	var current strings.Builder

	for _, seg := range segments {
		segLen := len(seg)
		if current.Len() == 0 && segLen <= c.MaxChunkLen {
			current.WriteString(seg)
			continue
		}

		// Add newline separator if not the first segment
		if current.Len()+segLen+1 <= c.MaxChunkLen {
			if current.Len() > 0 {
				current.WriteByte('\n')
			}
			current.WriteString(seg)
		} else {
			if current.Len() > 0 {
				chunks = append(chunks, current.String())
				current.Reset()
			}
			if segLen > c.MaxChunkLen {
				// Segment still too long, split further
				for _, sub := range c.splitBySpace(seg) {
					if len(sub) > c.MaxChunkLen {
						chunks = append(chunks, c.hardSplit(sub)...)
					} else {
						chunks = append(chunks, sub)
					}
				}
			} else {
				current.WriteString(seg)
			}
		}
	}

	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
}

// splitByParagraph splits at double newlines (paragraph boundaries).
func (c *Chunker) splitByParagraph(text string) []string {
	parts := strings.Split(text, "\n\n")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// splitBySpace splits at word boundaries.
func (c *Chunker) splitBySpace(text string) []string {
	var chunks []string
	words := strings.Fields(text)
	var current strings.Builder

	for _, word := range words {
		if current.Len() == 0 {
			current.WriteString(word)
			continue
		}
		if current.Len()+len(word)+1 <= c.MaxChunkLen {
			current.WriteByte(' ')
			current.WriteString(word)
		} else {
			chunks = append(chunks, current.String())
			current.Reset()
			current.WriteString(word)
		}
	}
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
}

// hardSplit splits at exactly MaxChunkLen with no regard for word boundaries.
func (c *Chunker) hardSplit(text string) []string {
	var chunks []string
	for len(text) > c.MaxChunkLen {
		chunks = append(chunks, text[:c.MaxChunkLen])
		text = text[c.MaxChunkLen:]
	}
	if len(text) > 0 {
		chunks = append(chunks, text)
	}
	return chunks
}
