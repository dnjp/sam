package main

const (
	MAXFILES    = 256
	READBUFSIZE = 8192
	NL          = 5
)

const (
	Up = iota
	Down
)

type Section struct {
	nrunes int
	text   []rune
	next   *Section
}

type Rasp struct {
	nrunes int
	sect   *Section
}

const Untagged = 0xFFFF

type Text struct {
	rasp      Rasp
	nwin      int
	front     int
	tag       int
	lock      int8
	l         [NL]Flayer
	id        int64
	tabwidth  int
	tabexpand bool
	comment   []rune
}

func (t *Text) Lock() {
	t.lock++
}

func (t *Text) Unlock() {
	t.lock--
}

func (t *Text) Locked() bool {
	return t.lock > 0
}

func (t *Text) NextLine(a0 int) int {
	for a0 < t.rasp.nrunes && raspc(&t.rasp, a0) != '\n' {
		a0++
	}
	a0++
	return a0
}

func (t *Text) MoveDown(a0 int) int {
	if a0 < t.rasp.nrunes {
		count := 0
		p0 := a0
		// find beginning of line
		for a0 > 0 && raspc(&t.rasp, a0-1) != '\n' {
			a0--
			// count how many characters we are from the beginning
			// of the line
			count++
		}
		a0 = p0
		// find end of line
		for a0 < t.rasp.nrunes && raspc(&t.rasp, a0) != '\n' {
			a0++
		}
		if a0 < t.rasp.nrunes {
			a0++ // skip past newline
			// move count number of characters forward on the next line
			for a0 < t.rasp.nrunes && count > 0 && raspc(&t.rasp, a0) != '\n' {
				a0++
				count--
			}
		}
	}
	return a0
}

func (t *Text) MoveUp(a0 int) int {
	if a0 > 0 {
		n0, n1, count := 0, 0, 0
		for a0 > 0 && raspc(&t.rasp, a0-1) != '\n' {
			a0--
			count++
		}
		if a0 > 0 {
			n1 = a0
			a0--
			for a0 > 0 && raspc(&t.rasp, a0-1) != '\n' {
				a0--
			}
			n0 = a0
			if n0+count >= n1 {
				a0 = n1 - 1
			} else {
				a0 = n0 + count
			}
		}
	}
	return a0
}

type Readbuf struct {
	n    int
	data [READBUFSIZE]uint8
}

type Resource int

const (
	RHost Resource = iota
	RKeyboard
	RMouse
	RPlumb
	RResize
	NRes
)
