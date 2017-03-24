package source

import "sort"

// Source is used with the lexer.
type Source struct {
	body       string
	name       string
	linesIndex []int // offset for each line: start offset of line n -> linesIndex[n-1] when line numbers start at 1
}

// Position represents a rune position in the source.
type Position struct {
	Offset int // offset, starting at 0
	Line   int // line number, starting at 1
	Column int // column number, starting at 1 (byte count)
}

// New initializes a new source with the provided name and body.
func New(name, body string) *Source {
	return &Source{
		name: name,
		body: body,
	}
}

// Name returns the name of the source
func (s *Source) Name() string {
	return s.name
}

// Body returns the body of the source
func (s *Source) Body() string {
	return s.body
}

// Position returns the line:column position from the provided absolute offset
func (s *Source) Position(offset int) Position {
	// Lazilly generate line index
	if len(s.linesIndex) == 0 {
		s.linesIndex = stringToLineIndex(s.body)
	}
	line := sort.SearchInts(s.linesIndex, offset+1)
	lineStart := s.linesIndex[len(s.linesIndex)-1]
	if line <= len(s.linesIndex) {
		lineStart = s.linesIndex[line-1]
	}
	return Position{
		Offset: offset,
		Line:   line,
		Column: offset - lineStart + 1,
	}
}

func stringToLineIndex(s string) []int {
	index := []int{0}
	var j int
	for _, r := range s {
		j++
		if r == '\n' {
			// Record start of next line
			index = append(index, j)
		}
	}
	return index
}
