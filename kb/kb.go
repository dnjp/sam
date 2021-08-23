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

func IndentSelection(in []rune, p0, p1, tw int, tabexpand, unindent bool) ([]rune, error) {
	out := []rune{}
	txt := in
	if len(txt) < p1-p0 {
		return out, fmt.Errorf("selection is greater than given input")
	}
	tab := Tab(tw, tabexpand)
	if !unindent {
		out = append(out, tab...)
	}
	for i := 0; i < len(txt); i++ {
		ch := txt[i]
		if ch == '\n' && i+1 != len(txt) && txt[i+1] != 0 {
			out = append(out, ch)
			if !unindent {
				out = append(out, tab...)
			}
		} else {
			if cont := stab(txt, i, tab); cont > 0 && unindent {
				if cont == 1 {
					continue
				} else {
					i += cont - 1
					continue
				}
			}
			out = append(out, ch)
		}
	}
	return out, nil
}

func CommentSelection(in []rune, filename string) ([]rune, error) {
	ft := FindFiletype(filename)
	comment := ft.Comment
	// parse starting/ending comment parts if present
	parts := strings.Split(strings.TrimSuffix(comment, " "), " ")
	multipart := len(parts) > 1
	var startcom string
	var endcom string
	if multipart {
		if len(parts[0]) > 0 {
			startcom = parts[0] + " "
		}
		if len(parts[1]) > 0 {
			endcom = " " + parts[1]
		}
	}
	rp := []rune{}
	for _, line := range linesfrom(in) {
		if len(line) <= 1 {
			rp = append(rp, line...)
			continue
		}
		linestr := string(line)
		if multipart {
			// uncomment multipart commented line
			hasbegin := strings.Contains(linestr, startcom)
			hasend := strings.Contains(linestr, endcom)
			if hasbegin && hasend {
				nline := strings.Replace(linestr, startcom, "", 1)
				nline = strings.Replace(nline, endcom, "", 1)
				rp = append(rp, []rune(nline)...)
				continue
			}
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

		// uncomment line if beginning charcters are the comment
		comstart := first + len(comment)
		if len(line) > comstart && linestr[first:comstart] == comment {
			nline := strings.Replace(linestr, comment, "", 1)
			rp = append(rp, []rune(nline)...)
			continue
		}

		// comment line using appropriate comment structure
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
