package kb

import (
	"fmt"
	"strings"
)

func Tab(tw int, tabexpand bool) []rune {
	tab := []rune{'\t'}
	if tabexpand {
		tab = []rune{}
		for i := 0; i < tw; i++ {
			tab = append(tab, ' ')
		}
	}
	return tab
}

func (ft Filetype) IndentSelection(in []rune, unindent bool) ([]rune, error) {
	tabwidth := ft.Tabwidth
	tabexpand := ft.Tabexpand
	tab := string(Tab(tabwidth, tabexpand))
	rp := []rune{}
	for _, line := range linesfrom(in) {
		tabs := 0
		spaces := 0
		first := 0
		offset := 0
		linestr := string(line)
		if len(line) < 1 || (len(line) == 1 && line[0] == '\n') {
			rp = append(rp, line...)
			continue
		}
		// find first non-indentation character
		for _, ch := range line {
			if ch == ' ' {
				first++
				spaces++
				continue
			} else if ch == '\t' {
				first++
				tabs++
				continue
			}
			break
		}
		// do not comment if we only have whitespace
		if first == len(line) {
			rp = append(rp, line...)
			continue
		}
		if unindent {
			tab = ""
			if tabs > spaces {
				offset = 1
			}
			if spaces > tabs {
				offset = tabwidth
			}
		}
		if offset > first {
			return rp, fmt.Errorf("improper offset for indentation")
		}
		rp = append(rp, []rune(linestr[offset:first]+tab+linestr[first:])...)
	}
	return rp, nil
}

func (ft Filetype) CommentSelection(in []rune) ([]rune, error) {
	comment := ft.Comment
	if comment[len(comment)-1] != ' ' {
		comment += " "
	}
	// parse starting/ending comment parts if present
	parts := ft.commentParts()
	multipart := len(parts) > 1
	startcom := ft.commentStart()
	endcom := ft.commentEnd()
	rp := []rune{}
	c := 0
	nc := 0
	lines := linesfrom(in)
	for _, line := range lines {
		if ft.HasComment(string(line)) {
			c++
		} else {
			nc++
		}
	}
	uncomment := c > nc
	for _, line := range lines {
		linestr := string(line)
		if len(line) < 1 || (len(line) == 1 && line[0] == '\n') {
			rp = append(rp, line...)
			continue
		}

		// find first non-indentation character
		first := 0
		for _, ch := range line {
			if ch == ' ' || ch == '\t' {
				first++
				continue
			}
			break
		}
		// do not comment if we only have whitespace
		if first == len(line) {
			rp = append(rp, line...)
			continue
		}

		if uncomment {
			if multipart {
				hasbegin := strings.Contains(linestr, startcom)
				hasend := strings.Contains(linestr, endcom)
				if hasbegin && hasend {
					nline := strings.Replace(linestr, startcom, "", 1)
					nline = strings.Replace(nline, endcom, "", 1)
					rp = append(rp, []rune(nline)...)
				}
				continue
			}
			comstart := first + len(comment)
			if len(line) > comstart && linestr[first:comstart] == comment {
				nline := strings.Replace(linestr, comment, "", 1)
				rp = append(rp, []rune(nline)...)
				continue
			}
			rp = append(rp, line...)
			continue
		}
		if multipart {
			end := len(line)
			if line[end-1] == '\n' {
				end = end - 1
			}
			l := []rune(linestr[:first] + startcom + linestr[first:end] + endcom)
			if l[len(l)-1] != '\n' {
				l = append(l, '\n')
			}
			rp = append(rp, l...)
			continue
		}
		rp = append(rp, []rune(linestr[:first]+comment+linestr[first:])...)
	}
	return rp, nil
}

func linesfrom(text []rune) [][]rune {
	lines := [][]rune{}
	start := 0
	for i := start; i < len(text); i++ {
		if text[i] == '\n' {
			lines = append(lines, text[start:i+1])
			start = i + 1
		}
		if i+1 == len(text) && text[i] != '\n' {
			lines = append(lines, text[start:])
		}
	}
	if start == 0 {
		lines = append(lines, text)
	}
	return lines
}

func stab(text []rune, i int, tab []rune) int {
	txtlen := len(text)
	tablen := len(tab)
	// i is out of bounds
	if (tablen == 1 && i > txtlen) || (tablen > 1 && i+(tablen-1) > txtlen) {
		return -1
	}
	// not at the start of the line
	if i > 0 && text[i-1] != '\n' {
		return -1
	}
	found := 0
	for j := i; j < i+tablen; j++ {
		if text[j] == '\t' || text[j] == ' ' {
			found++
		}
	}
	if found < len(tab) {
		return -1
	}
	return found
}
