package kb

import "fmt"

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
