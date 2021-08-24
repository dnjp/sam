package main

import (
	"fmt"
	"image"
	"os"
	"os/signal"
	"strings"

	"github.com/dnjp/9fans/draw"
	"github.com/dnjp/sam/kb"
	"github.com/dnjp/sam/mesg"
)

var logfile = func() *os.File {
	f, err := os.OpenFile("/tmp/samterm.out", os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	return f
}()

func logf(fmtstr string, args ...interface{}) {
	logfile.Write([]byte(fmt.Sprintf(fmtstr, args...)))
}

var (
	cmd         Text
	cursor      *draw.Cursor
	which       *Flayer
	work        *Flayer
	snarflen    int
	typestart   int  = -1
	typeend     int  = -1
	typeesc     int  = -1
	modified    bool /* strange lookahead for menus */
	hostlock    int  = 1
	hasunlocked bool
	maxtab      int = 8
	chord       int
	autoindent  bool
	display     *draw.Display
	screen      *draw.Image
	font        *draw.Font
	textID      int64
	textByID    map[int64]*Text
)

const chording = true /* code here for reference but it causes deadlocks */

func main() {
	/*
	 * sam is talking to us on fd 0 and 1.
	 * move these elsewhere so that if we accidentally
	 * use 0 and 1 in other code, nothing bad happens.
	 */
	hostfd[0] = os.Stdin
	hostfd[1] = os.Stdout
	os.Stdin, _ = os.Open(os.DevNull)
	os.Stdout = os.Stderr

	// ignore interrupt signals
	signal.Notify(make(chan os.Signal), os.Interrupt)

	if protodebug {
		print("getscreen\n")
	}
	getscreen()
	if protodebug {
		print("iconinit\n")
	}
	iconinit()
	if protodebug {
		print("initio\n")
	}
	initio()
	if protodebug {
		print("scratch\n")
	}
	r := screen.R
	r.Max.Y = r.Min.Y + r.Dy()/5
	if protodebug {
		print("flstart\n")
	}
	flstart(screen.Clipr)
	rinit(&cmd.rasp)
	flnew(&cmd.l[0], gettext, &cmd)
	flinit(&cmd.l[0], r, font, cmdcols[:])
	textID++
	cmd.id = textID
	textByID = make(map[int64]*Text)
	textByID[cmd.id] = &cmd
	cmd.nwin = 1
	which = &cmd.l[0]
	cmd.tag = Untagged
	outTs(mesg.Tversion, mesg.VERSION)
	startnewfile(mesg.Tstartcmdfile, &cmd)

	got := 0
	if protodebug {
		print("loop\n")
	}
	for ; ; got = waitforio() {
		if hasunlocked && RESIZED() {
			resize()
		}
		if got&(1<<RHost) != 0 {
			rcv()
		}
		if got&(1<<RPlumb) != 0 {
			var i int
			for i = 0; cmd.l[i].textfn == nil; i++ {
			}
			current(&cmd.l[i])
			flsetselect(which, cmd.rasp.nrunes, cmd.rasp.nrunes)
			ktype(which, RPlumb)
		}
		if got&(1<<RKeyboard) != 0 {
			if which != nil {
				ktype(which, RKeyboard)
			} else {
				kbdblock()
			}
		}
		if got&(1<<RMouse) != 0 {
			if hostlock == 2 || !mousep.Point.In(screen.R) {
				mouseunblock()
				continue
			}
			nwhich := flwhich(mousep.Point)
			scr := which != nil && mousep.Point.In(which.scroll)
			scr = which != nil && (mousep.Point.In(which.scroll)) ||
				mousep.Buttons&(8|16) != 0
			if mousep.Buttons != 0 {
				flushtyping(true)
			}
			if chording && chord == 1 && mousep.Buttons == 0 {
				chord = 0
			}
			if chording && chord != 0 {
				chord |= mousep.Buttons
			} else if mousep.Buttons&(1|8) != 0 {
				if nwhich != nil {
					if nwhich != which {
						current(nwhich)
					} else if scr {
						scroll(which, func() int {
							if mousep.Buttons&8 != 0 {
								return 4
							}
							return 1
						}())
					} else {
						t := which.text
						nclick := flselect(which)
						if nclick > 0 {
							if nclick > 1 {
								outTsl(mesg.Ttclick, t.tag, which.p0)
								t.lock++
							} else {
								outTsl(mesg.Tdclick, t.tag, which.p0)
								t.lock++
							}
						} else if t != &cmd {
							outcmd()
						}
						if mousep.Buttons&1 != 0 {
							chord = mousep.Buttons
						}
					}
				}
			} else if mousep.Buttons&2 != 0 && which != nil {
				if scr {
					scroll(which, 2)
				} else {
					menu2hit()
				}
			} else if mousep.Buttons&(4|16) != 0 {
				if scr {
					scroll(which, func() int {
						if mousep.Buttons&16 != 0 {
							return 5
						}
						return 3
					}())
				} else {
					menu3hit()
				}
			}
			mouseunblock()
		}
		if chording && chord != 0 {
			t := which.text
			if t.lock == 0 && hostlock == 0 {
				w := t.find(which)
				if chord&2 != 0 {
					cut(t, w, true, true)
					chord &= ^2
				} else if chord&4 != 0 {
					paste(t, w)
					chord &= ^4
				}
			}
		}
	}
}

func (t *Text) find(l *Flayer) int {
	w := 0
	for &t.l[w] != l {
		w++
	}
	return w
}

func resize() {
	flresize(screen.Clipr)
	for _, t := range text {
		if t != nil {
			hcheck(t.tag)
		}
	}
}

func current(nw *Flayer) {
	if which != nil {
		flborder(which, false)
	}
	if nw != nil {
		flushtyping(true)
		flupfront(nw)
		flborder(nw, true)
		buttons(Up)
		t := nw.text
		t.front = t.find(nw)
		if t != &cmd {
			work = nw
		}
	}
	which = nw
}

func closeup(l *Flayer) {
	t := l.text
	m := whichmenu(t.tag)
	if m < 0 {
		return
	}
	flclose(l)
	if l == which {
		which = nil
		current(flwhich(image.Pt(0, 0)))
	}
	if l == work {
		work = nil
	}
	t.nwin--
	if t.nwin == 0 {
		rclear(&t.rasp)
		delete(textByID, t.id)
		free(t)
		text[m] = nil
	} else if l == &t.l[t.front] {
		for m = 0; m < NL; m++ { /* find one; any one will do */
			if t.l[m].textfn != nil {
				t.front = m
				return
			}
		}
		panic("close")
	}
}

func findl(t *Text) *Flayer {
	for i := 0; i < NL; i++ {
		if t.l[i].textfn == nil {
			return &t.l[i]
		}
	}
	return nil
}

func duplicate(l *Flayer, r image.Rectangle, f *draw.Font, close bool) {
	t := l.text
	nl := findl(t)
	if nl != nil {
		flnew(nl, gettext, t)
		flinit(nl, r, f, l.f.Cols[:])
		nl.origin = l.origin
		rp := l.textfn(l, l.f.NumChars)
		flinsert(nl, rp, l.origin)
		flsetselect(nl, l.p0, l.p1)
		if close {
			flclose(l)
			if l == which {
				which = nil
			}
		} else {
			t.nwin++
		}
		current(nl)
		hcheck(t.tag)
	}
	display.SwitchCursor(cursor)
}

func buttons(updown int) {
	for (mousep.Buttons&7 != 0) != (updown == Down) {
		getmouse()
	}
}

func getr(rp *image.Rectangle) bool {
	*rp = draw.SweepRect(3, mousectl)
	if rp.Max.X != 0 && rp.Max.X-rp.Min.X <= 5 && rp.Max.Y-rp.Min.Y <= 5 {
		p := rp.Min
		r := cmd.l[cmd.front].entire
		*rp = screen.R
		if cmd.nwin == 1 {
			if p.Y <= r.Min.Y {
				rp.Max.Y = r.Min.Y
			} else if p.Y >= r.Max.Y {
				rp.Min.Y = r.Max.Y
			}
			if p.X <= r.Min.X {
				rp.Max.X = r.Min.X
			} else if p.X >= r.Max.X {
				rp.Min.X = r.Max.X
			}
		}
	}
	return draw.RectClip(rp, screen.R) && rp.Max.X-rp.Min.X > 100 && rp.Max.Y-rp.Min.Y > 40
}

func snarf(t *Text, w int) {
	l := &t.l[w]
	if l.p1 > l.p0 {
		snarflen = l.p1 - l.p0
		outTsll(mesg.Tsnarf, t.tag, l.p0, l.p1)
	}
}

func cut(t *Text, w int, save bool, check bool) {
	l := &t.l[w]
	p0 := l.p0
	p1 := l.p1
	if p0 == p1 {
		return
	}
	if p0 < 0 {
		panic("cut")
	}
	if save {
		snarf(t, w)
	}
	outTsll(mesg.Tcut, t.tag, p0, p1)
	flsetselect(l, p0, p0)
	t.lock++
	hcut(t.tag, p0, p1-p0)
	if check {
		hcheck(t.tag)
	}
}

func paste(t *Text, w int) {
	if snarflen != 0 {
		cut(t, w, false, false)
		t.lock++
		outTsl(mesg.Tpaste, t.tag, t.l[w].p0)
	}
}

func scrorigin(l *Flayer, but int, p0 int) {
	t := l.text

	if t.tag == Untagged {
		return
	}

	switch but {
	case 1:
		outTsll(mesg.Torigin, t.tag, l.origin, p0)
	case 2:
		outTsll(mesg.Torigin, t.tag, p0, 1)
	case 3:
		horigin(t.tag, p0)
	}
}

func alnum(c rune) bool {
	/*
	 * Hard to get absolutely right.  Use what we know about ASCII
	 * and assume anything above the Latin control characters is
	 * potentially an alphanumeric.
	 */
	if c <= ' ' {
		return false
	}
	if 0x7F <= c && c <= 0xA0 {
		return false
	}
	if strings.ContainsRune("!\"#$%&'()*+,-./:;<=>?@[\\]^`{|}~", c) {
		return false
	}
	return true
}

func raspc(r *Rasp, p int) rune {
	nr := rload(r, p, p+1)
	if len(nr) > 0 {
		return nr[0]
	}
	return 0
}

func ctlw(r *Rasp, o int, p int) int {
	p--
	if p < o {
		return o
	}
	if raspc(r, p) == '\n' {
		return p
	}
	for ; p >= o; p-- {
		c := raspc(r, p)
		if alnum(c) {
			break
		}
		if c == '\n' {
			return p + 1
		}
	}
	for ; p > o && alnum(raspc(r, p-1)); p-- {
	}
	if p >= o {
		return p
	}
	return o
}

func ctlu(r *Rasp, o int, p int) int {
	p--
	if p < o {
		return o
	}
	if raspc(r, p) == '\n' {
		return p
	}
	for ; p-1 >= o && raspc(r, p-1) != '\n'; p-- {
	}
	if p >= o {
		return p
	}
	return o
}

func center(l *Flayer, a int) bool {
	t := l.text
	if a < l.origin || shouldscroll(l, a) {
		if a > t.rasp.nrunes {
			a = t.rasp.nrunes
		}
		outTsll(mesg.Torigin, t.tag, a, 2)
		return true
	}
	return false
}

func shouldscroll(l *Flayer, a int) bool {
	// do not overflow command window
	if l == &cmd.l[0] {
		py := float64(l.f.PointOf(l.origin + l.f.NumChars).Y)
		my := float64(l.f.Entire.Max.Y)
		return py/my >= 0.90
	} else {
		return l.origin+l.f.NumChars < a
	}
}

func thirds(l *Flayer, a int, n int) bool {
	t := l.text
	if a < l.origin || shouldscroll(l, a) {
		if a > t.rasp.nrunes {
			a = t.rasp.nrunes
		}
		s := l.scroll.Inset(1)
		lines := (n*(s.Max.Y-s.Min.Y)/l.f.Font.Height + 1) / 3
		if lines < 2 {
			lines = 2
		}
		outTsll(mesg.Torigin, t.tag, a, lines)
		return true
	}
	return false
}

func onethird(l *Flayer, a int) bool {
	return thirds(l, a, 1)
}

func twothirds(l *Flayer, a int) bool {
	return thirds(l, a, 2)
}

// flushtyping sets the current state based on
// what was typed.
func flushtyping(clearesc bool) {
	if clearesc {
		typeesc = -1
	}
	if typestart == typeend {
		modified = false
		return
	}
	t := which.text
	if t != &cmd {
		modified = true
	}
	// retrieve typed text from rasp
	rp := rload(&t.rasp, typestart, typeend)
	// if the last character typed is at the end of the current rasp and
	// we're at a newline,
	if t == &cmd && typeend == t.rasp.nrunes && rp[len(rp)-1] == '\n' {
		setlock() // sets hostlock
		outcmd()  // sends `work` to host
	}
	outTslS(mesg.Ttype, t.tag, typestart, rp) // send typed text to host
	typestart = -1
	typeend = -1
}

func kout(l *Flayer, t *Text, a int, atnl bool, in []rune) {
	if len(in) <= 0 {
		return
	}
	if typestart < 0 {
		typestart = a
	}
	if typeesc < 0 {
		typeesc = a
	}
	// grows rasp and inserts the data between a:len(kinput) into the
	// rasp (retrieved from the host)
	hgrow(t.tag, a, len(in), 0)
	t.lock++                // pretend we Trequest'ed for hdatarune
	hdatarune(t.tag, a, in) // insert a:len(kinput) into rasp
	a += len(in)
	l.p0 = a
	l.p1 = a
	typeend = a
	if atnl || typeend-typestart > 100 {
		flushtyping(false)
	}
	onethird(l, a) // make sure the text is visible
}

const (
	BACKSCROLLKEY = draw.KeyUp
	ENDKEY        = draw.KeyEnd
	ESC           = '\x1B'
	HOMEKEY       = draw.KeyHome
	LEFTARROW     = draw.KeyLeft
	LINEEND       = '\x05'
	LINESTART     = '\x01'
	PAGEDOWN      = draw.KeyPageDown
	PAGEUP        = draw.KeyPageUp
	RIGHTARROW    = draw.KeyRight
	SCROLLKEY     = draw.KeyDown
	BACKC         = 0x02 // ctrl+b
	FWDC          = 0x06 // ctrl+f
	DOWNC         = 0x0E // ctrl+n
	UPC           = 0x10 // ctrl+p
	CUT           = draw.KeyCmd + 'x'
	COPY          = draw.KeyCmd + 'c'
	PASTE         = draw.KeyCmd + 'v'
	UNDO          = draw.KeyCmd + 'z'
	BACK          = 0x14 // ctrl+t
	LAST          = 0x07 // ctrl+g
	INDENT        = '\t'
	UNINDENT      = 0x19 // shift+tab
	COMMENT       = draw.KeyCmd + '/'
)

func nontypingkey(c rune) bool {
	switch c {
	case BACKSCROLLKEY,
		ENDKEY,
		HOMEKEY,
		LEFTARROW,
		LINEEND,
		LINESTART,
		PAGEDOWN,
		PAGEUP,
		RIGHTARROW,
		SCROLLKEY,
		BACKC,
		FWDC,
		DOWNC,
		UPC,
		CUT,
		COPY,
		PASTE,
		UNDO,
		BACK,
		LAST,
		COMMENT:
		return true
	}
	return false
}

var indentq []rune
var kinput = make([]rune, 0, 100)

func ktype(l *Flayer, res Resource) {
	var c rune
	t := l.text
	scrollkey := false

	if res == RKeyboard {
		c = qpeekc() /* ICK */
		scrollkey = nontypingkey(c)
	}

	if hostlock != 0 || t.lock != 0 {
		kbdblock()
		return
	}
	a := l.p0
	kinput = kinput[:0]
	backspacing := 0

	if a != l.p1 && !scrollkey {
		if c == INDENT || c == UNINDENT {
			// without this, the initial tab event causes
			// a continuous cycle because `got` is never
			// reset. this is probably a bug.
			got = 0
			goto Out
		}
		flushtyping(true)
		cut(t, t.front, true, true)
		return /* it may now be locked */
	}

	for {
		c = kbdchar()
		if c <= 0 {
			break
		}
		if res == RKeyboard {
			if nontypingkey(c) || c == ESC || c == INDENT || c == UNINDENT {
				break
			}
			/* backspace, ctrl-u, ctrl-w, del */
			if c == '\b' || c == 0x15 || c == 0x17 || c == 0x7F {
				backspacing = 1
				break
			}
		}
		kinput = append(kinput, c)
		if autoindent {
			if c == '\n' {
				cursor := ctlu(&t.rasp, 0, a+len(kinput)-1)
				for len(kinput) < cap(kinput) {
					ch := raspc(&t.rasp, cursor)
					cursor++
					if ch == ' ' || ch == '\t' {
						kinput = append(kinput, ch)
					} else {
						break
					}
				}
			}
		}
		if c == '\n' || len(kinput) == cap(kinput) {
			break
		}
	}
	kout(l, t, a, c == '\n', kinput)
Out:
	switch c {
	case INDENT, UNINDENT:
		if l.p1 > l.p0 {
			flushtyping(false)
			cut(t, t.front, true, false)
			t.lock++
			if c == UNINDENT {
				outTsl(mesg.Tunindent, t.tag, l.p0)
			} else {
				outTsl(mesg.Tindent, t.tag, l.p0)
			}
		} else if c == INDENT {
			kout(l, t, a, false, kb.Tab(l.text.tabwidth, l.tabexpand))
		}
	case COMMENT:
		flushtyping(false)
		cut(t, t.front, true, false)
		t.lock++
		outTsl(mesg.Tcomment, t.tag, l.p0)
	case SCROLLKEY, PAGEDOWN:
		flushtyping(false)
		center(l, l.origin+l.f.NumChars+1)
	case BACKSCROLLKEY, PAGEUP:
		flushtyping(false)
		a0 := l.origin - l.f.NumChars
		if a0 < 0 {
			a0 = 0
		}
		center(l, a0)
	case RIGHTARROW, FWDC:
		flushtyping(false)
		a0 := l.p0
		if a0 < t.rasp.nrunes {
			a0++
		}
		flsetselect(l, a0, a0)
		center(l, a0)
	case LEFTARROW, BACKC:
		flushtyping(false)
		a0 := l.p0
		if a0 > 0 {
			a0--
		}
		flsetselect(l, a0, a0)
		center(l, a0)
	case UPC:
		a0 := l.p0
		flsetselect(l, a0, a0)
		flushtyping(true)
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
				flsetselect(l, a0, a0)
				center(l, a0)
			}
		}
	case DOWNC:
		a0 := l.p0
		flsetselect(l, a0, a0)
		flushtyping(true)
		if a0 < t.rasp.nrunes {
			count := 0
			p0 := a0
			for a0 > 0 && raspc(&t.rasp, a0-1) != '\n' {
				a0--
				count++
			}
			a0 = p0
			for a0 < t.rasp.nrunes && raspc(&t.rasp, a0) != '\n' {
				a0++
			}
			if a0 < t.rasp.nrunes {
				a0++
				for a0 < t.rasp.nrunes && count > 0 && raspc(&t.rasp, a0) != '\n' {
					a0++
					count--
				}
				if a0 != p0 {
					flsetselect(l, a0, a0)
					center(l, a0)
				}
			}
		}
	case HOMEKEY:
		flushtyping(false)
		center(l, 0)
	case ENDKEY:
		flushtyping(false)
		center(l, t.rasp.nrunes)
	case LINESTART, LINEEND:
		flushtyping(true)
		if c == LINESTART {
			for a > 0 && raspc(&t.rasp, a-1) != '\n' {
				a--
			}
		} else {
			for a < t.rasp.nrunes && raspc(&t.rasp, a) != '\n' {
				a++
			}
		}
		l.p1 = a
		l.p0 = l.p1
		for i := 0; i < NL; i++ {
			l := &t.l[i]
			if l.textfn != nil {
				flsetselect(l, l.p0, l.p1)
			}
		}
	case UNDO:
		flushtyping(false)
		outTs(mesg.Tundo, t.tag)
	default:
		if backspacing != 0 && hostlock == 0 {
			/* backspacing immediately after outcmd(): sorry */
			if l.f.P0 > 0 && a > 0 {
				switch c {
				case '\b',
					0x7F: /* del */
					l.p0 = a - 1
				case 0x15: /* ctrl-u */
					l.p0 = ctlu(&t.rasp, l.origin, a)
				case 0x17: /* ctrl-w */
					l.p0 = ctlw(&t.rasp, l.origin, a)
				}
				l.p1 = a
				if l.p1 != l.p0 {
					/* cut locally if possible */
					if typestart <= l.p0 && l.p1 <= typeend {
						t.lock++ /* to call hcut */
						hcut(t.tag, l.p0, l.p1-l.p0)
						/* hcheck is local because we know rasp is contiguous */
						hcheck(t.tag)
					} else {
						flushtyping(false)
						cut(t, t.front, false, true)
					}
				}
				if typeesc >= l.p0 {
					typeesc = l.p0
				}
				if typestart >= 0 {
					if typestart >= l.p0 {
						typestart = l.p0
					}
					typeend = l.p0
					if typestart == typeend {
						typestart = -1
						typeend = -1
						modified = false
					}
				}
			}
		} else {
			var i int
			if c == ESC && typeesc >= 0 {
				l.p0 = typeesc
				l.p1 = a
				flushtyping(true)
			}
			for i := 0; i < NL; i++ {
				l := &t.l[i]
				if l.textfn != nil {
					flsetselect(l, l.p0, l.p1)
				}
			}
			switch c {
			case CUT:
				flushtyping(false)
				cut(t, t.front, true, true)
			case COPY:
				flushtyping(false)
				snarf(t, t.front)
			case PASTE:
				flushtyping(false)
				paste(t, t.front)
			case BACK:
				t = &cmd
				for i := 0; i < len(t.l); i++ {
					if flinlist(&t.l[i]) {
						l = &t.l[i]
					}
				}
				current(l)
				flushtyping(false)
				a = t.rasp.nrunes
				flsetselect(l, a, a)
				center(l, a)
			case LAST:
				if work == nil {
					return
				}
				if which != work {
					current(work)
					return
				}
				t = work.text
				l = &t.l[t.front]
				for i = t.front; t.nwin > 1 && incr(&i) != t.front; {
					if t.l[i].textfn != nil {
						l = &t.l[i]
						break
					}
				}
				current(l)
				break
			}
		}
	}
}

func incr(v *int) int {
	*v = (*v + 1) % NL
	return *v
}

func outcmd() {
	if work != nil {
		outTsll(mesg.Tworkfile, work.text.tag, work.p0, work.p1)
	}
}

func gettext(l *Flayer, n int) []rune {
	return rload(&l.text.rasp, l.origin, l.origin+n)
}

func scrtotal(l *Flayer) int {
	return l.text.rasp.nrunes
}
