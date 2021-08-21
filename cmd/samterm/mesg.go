package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"unicode/utf8"

	"github.com/dnjp/9fans/plumb"
	"github.com/dnjp/sam/mesg"
)

const HSIZE = 3 /* Type + short count */
var h mesg.Header
var indata = make([]byte, 0, mesg.DATASIZE)
var outdata [mesg.DATASIZE]byte
var outcount int
var hversion int
var hostfd [2]*os.File
var exiting int

var rcv_state int = 0
var rcv_count int = 0
var rcv_errs int = 0

func rcv() {
	if protodebug {
		print("rcv in\n")
	}
	for c := rcvchar(); c >= 0; c = rcvchar() {
		if protodebug {
			print(".")
		}
		switch rcv_state {
		case 0:
			h.Typ = mesg.Hmesg(c).Wire()
			rcv_state++

		case 1:
			h.Count0 = byte(c)
			rcv_state++

		case 2:
			h.Count1 = byte(c)
			rcv_count = int(h.Count0) | int(h.Count1)<<8
			if rcv_count > mesg.DATASIZE {
				rcv_errs++
				if rcv_errs < 5 {
					dumperrmsg(rcv_count, mesg.Hmesg(h.Typ), int(h.Count0), c)
					rcv_state = 0
					continue
				}
				fmt.Fprintf(os.Stderr, "type %d count %d\n", h.Typ, rcv_count)
				panic("count>mesg.DATASIZE")
			}
			indata = indata[:0]
			if rcv_count == 0 {
				inmesg(mesg.Hmesg(h.Typ), 0)
				rcv_count = 0
				rcv_state = 0
				continue
			}
			rcv_state++

		case 3:
			indata = append(indata, byte(c))
			if len(indata) == rcv_count {
				inmesg(mesg.Hmesg(h.Typ), rcv_count)
				rcv_count = 0
				rcv_state = 0
				continue
			}
		}
		if protodebug {
			print(":")
		}
	}

	if protodebug {
		print("rcv out\n")
	}
}

func whichtext(tg int) *Text {
	for i := range tag {
		if tag[i] == tg {
			return text[i]
		}
	}
	println("TEXT")
	for i := range tag {
		println(tag[i], text[i], string(name[i]))
	}
	panic("whichtext")
	// return nil
}

func inmesg(typ mesg.Hmesg, count int) {
	m := inshort(0)
	l := inlong(2)
	switch typ {
	case -1:
		panic("rcv error")
		// fallthrough
	default:
		fmt.Fprintf(os.Stderr, "type %d\n", typ)
		panic("rcv unknown")
		// fallthrough

	case mesg.Hversion:
		hversion = m

	case mesg.Htabwidth:
		l := invlong(2)
		i := whichmenu(m)
		t := whichtext(m)
		if i < 0 || t == nil {
			panic("NOT CURRENT")
			break
		}
		t.tabwidth = int(l)
		lp := &t.l[t.front]
		if t.l[t.front].textfn != nil {
			lp.f.MaxTab = int(l * int64(lp.f.Font.StringWidth("0")))
		}
		break

	case mesg.Htabexpand:
		i := whichmenu(m)
		t := whichtext(m)
		if i < 0 || t == nil {
			break
		}
		lp := &t.l[t.front]
		if t.l[t.front].textfn == nil {
			break
		}
		if lp.tabexpand {
			lp.tabexpand = false
		} else {
			lp.tabexpand = true
		}
		break

	case mesg.Hbindname:
		l := invlong(2) /* for 64-bit pointers */
		i := whichmenu(m)
		if i < 0 {
			break
		}
		/* in case of a race, a bindname may already have occurred */
		old := textByID[l]
		t := whichtext(m)
		if t == nil {
			t = old
		} else { /* let the old one win; clean up the new one */
			for old.nwin > 0 {
				closeup(&old.l[old.front])
			}
		}
		text[i] = t
		text[i].tag = m

	case mesg.Hcurrent:
		if whichmenu(m) < 0 {
			break
		}
		t := whichtext(m)
		isCmd := which != nil && which.text == &cmd && m != cmd.tag
		if t == nil {
			t = sweeptext(false, m)
			if t == nil {
				break
			}
		}
		if t.l[t.front].textfn == nil {
			panic("Hcurrent")
		}
		lp := &t.l[t.front]
		if isCmd {
			flupfront(lp)
			flborder(lp, false)
			work = lp
		} else {
			current(lp)
		}

	case mesg.Hmovname:
		m := whichmenu(m)
		if m < 0 {
			break
		}
		t := text[m]
		l := tag[m]
		i := name[m][0]
		text[m] = nil /* suppress panic in menudel */
		menudel(m)
		if t == &cmd {
			m = 0
		} else {
			if len(text) > 0 && text[0] == &cmd {
				m = 1
			} else {
				m = 0
			}
			for ; m < len(name); m++ {
				if bytes.Compare(indata[2:], name[m][1:]) < 0 {
					break
				}
			}
		}
		menuins(m, indata[2:], t, i, int(l))

	case mesg.Hgrow:
		if whichmenu(m) >= 0 {
			hgrow(m, l, inlong(6), 1)
		}

	case mesg.Hnewname:
		menuins(0, nil, nil, ' ', m)

	case mesg.Hcheck0:
		i := whichmenu(m)
		if i >= 0 {
			t := text[i]
			if t != nil {
				t.lock++
			}
			outTs(mesg.Tcheck, m)
		}

	case mesg.Hcheck:
		i := whichmenu(m)
		if i >= 0 {
			t := text[i]
			if t != nil && t.lock != 0 {
				t.lock--
			}
			hcheck(m)
		}

	case mesg.Hunlock:
		clrlock()

	case mesg.Hdata:
		if whichmenu(m) >= 0 {
			l += hdata(m, l, indata[6:])
		}
		goto Checkscroll

	case mesg.Horigin:
		if whichmenu(m) >= 0 {
			horigin(m, l)
		}

	case mesg.Hunlockfile:
		if whichmenu(m) >= 0 {
			t := whichtext(m)
			if t.lock != 0 {
				t.lock--
				l = -1
				goto Checkscroll
			}
		}

	case mesg.Hsetdot:
		if whichmenu(m) >= 0 {
			hsetdot(m, l, inlong(6))
		}

	case mesg.Hgrowdata:
		if whichmenu(m) < 0 {
			break
		}
		hgrow(m, l, inlong(6), 0)
		whichtext(m).lock++ /* fake the request */
		l += hdata(m, l, indata[10:])
		goto Checkscroll

	case mesg.Hmoveto:
		if whichmenu(m) >= 0 {
			hmoveto(m, l)
		}

	case mesg.Hclean:
		m := whichmenu(m)
		if m >= 0 {
			name[m][0] = ' '
		}

	case mesg.Hdirty:
		m := whichmenu(m)
		if m >= 0 {
			name[m][0] = '\''
		}

	case mesg.Hdelname:
		m := whichmenu(m)
		if m >= 0 {
			menudel(m)
		}

	case mesg.Hcut:
		if whichmenu(m) >= 0 {
			hcut(m, l, inlong(6))
		}

	case mesg.Hclose:
		if whichmenu(m) < 0 {
			break
		}
		t := whichtext(m)
		if t == nil {
			break
		}
		l := t.nwin
		for i := 0; l > 0 && i < NL; i++ {
			lp := &t.l[i]
			if lp.textfn != nil {
				closeup(lp)
				l--
			}
		}

	case mesg.Hsetpat:
		setpat(indata)

	case mesg.Hsetsnarf:
		hsetsnarf(m)

	case mesg.Hsnarflen:
		snarflen = inlong(0)

	case mesg.Hack:
		outT0(mesg.Tack)

	case mesg.Hexit:
		exiting = 1
		outT0(mesg.Texit)
		os.Exit(0)

	case mesg.Hplumb:
		hplumb(m)
	}
	return

Checkscroll:
	if m == cmd.tag {
		for i := 0; i < NL; i++ {
			lp := &cmd.l[i]
			if lp.textfn != nil {
				p := int(l)
				if p < 0 {
					p = lp.p1
				}
				center(lp, p)
			}
		}
	}
}

// setlock sets the hostlock and switches to the
// `lockarrow` cursor
func setlock() {
	hostlock++
	cursor = &lockarrow
	display.SwitchCursor(cursor)
}

func clrlock() {
	hasunlocked = true
	if hostlock > 0 {
		hostlock--
	}
	if hostlock == 0 {
		cursor = nil
		display.SwitchCursor(cursor)
	}
}

func startfile(t *Text) {
	outTsv(mesg.Tstartfile, t.tag, t.id) /* for 64-bit pointers */
	setlock()
}

func startnewfile(typ mesg.Tmesg, t *Text) {
	t.tag = Untagged
	t.tabwidth = maxtab
	outTv(typ, t.id) /* for 64-bit pointers */
}

func inshort(n int) int {
	return int(binary.LittleEndian.Uint16(indata[n : n+2]))
}

func inlong(n int) int {
	return int(binary.LittleEndian.Uint32(indata[n : n+4]))
}

func invlong(n int) int64 {
	return int64(binary.LittleEndian.Uint64(indata[n : n+8]))
}

func outT0(typ mesg.Tmesg) {
	outstart(typ)
	outsend()
}

func outTl(typ mesg.Tmesg, l int) {
	outstart(typ)
	outlong(l)
	outsend()
}

func outTs(typ mesg.Tmesg, s int) {
	outstart(typ)
	outshort(s)
	outsend()
}

func outTss(typ mesg.Tmesg, s1 int, s2 int) {
	outstart(typ)
	outshort(s1)
	outshort(s2)
	outsend()
}

func outTsll(typ mesg.Tmesg, s1 int, l1 int, l2 int) {
	outstart(typ)
	outshort(s1)
	outlong(l1)
	outlong(l2)
	outsend()
}

func outTsl(typ mesg.Tmesg, s1 int, l1 int) {
	outstart(typ)
	outshort(s1)
	outlong(l1)
	outsend()
}

func outTsv(typ mesg.Tmesg, s1 int, v1 int64) {
	outstart(typ)
	outshort(s1)
	outvlong(v1)
	outsend()
}

func outTv(typ mesg.Tmesg, v1 int64) {
	outstart(typ)
	outvlong(v1)
	outsend()
}

func outTslS(typ mesg.Tmesg, s1 int, l1 int, s []rune) {
	outstart(typ)
	outshort(s1)
	outlong(l1)
	outrunes(s)
	outsend()
}

func outTsls(typ mesg.Tmesg, s1 int, l1 int, s2 int) {
	outstart(typ)
	outshort(s1)
	outlong(l1)
	outshort(s2)
	outsend()
}

func outstart(typ mesg.Tmesg) {
	outdata[0] = byte(typ)
	outcount = 0
}

func outrunes(s []rune) {
	for _, r := range s {
		outcount += utf8.EncodeRune(outdata[HSIZE+outcount:HSIZE+outcount+utf8.UTFMax], r)
	}
}

func outshort(s int) {
	binary.LittleEndian.PutUint16(outdata[HSIZE+outcount:HSIZE+outcount+2], uint16(s))
	outcount += 2
}

func outlong(l int) {
	binary.LittleEndian.PutUint32(outdata[HSIZE+outcount:HSIZE+outcount+4], uint32(l))
	outcount += 4
}

func outvlong(v int64) {
	binary.LittleEndian.PutUint64(outdata[HSIZE+outcount:HSIZE+outcount+8], uint64(v))
	outcount += 8
}

// outsend sends the `outdata` to the hosts file descriptor
func outsend() {
	if outcount > mesg.DATASIZE-HSIZE {
		panic("outcount>sizeof outdata")
	}
	outdata[1] = uint8(outcount)
	outdata[2] = uint8(outcount >> 8)
	if n, err := hostfd[1].Write(outdata[:outcount+HSIZE]); n != int(outcount+HSIZE) {
		panic("write error: " + err.Error())
	}
}

func hsetdot(m int, p0 int, p1 int) {
	t := whichtext(m)
	l := &t.l[t.front]

	flushtyping(true)
	flsetselect(l, p0, p1)
}

func horigin(m int, p0 int) {
	t := whichtext(m)
	l := &t.l[t.front]
	if !flprepare(l) {
		l.origin = p0
		return
	}
	a := p0 - l.origin
	if a >= 0 && a < l.f.NumChars {
		l.f.Delete(0, a)
	} else if a < 0 && -a < l.f.NumChars {
		rp := rload(&t.rasp, p0, l.origin)
		l.f.Insert(rp, 0)
	} else {
		l.f.Delete(0, l.f.NumChars)
	}
	l.origin = p0
	scrdraw(l, t.rasp.nrunes)
	if l.visible == Some {
		flrefresh(l, l.entire, 0)
	}
	hcheck(m)
}

func hmoveto(m int, p0 int) {
	t := whichtext(m)
	l := &t.l[t.front]

	if p0 < l.origin || p0-l.origin > l.f.NumChars*9/10 {
		outTsll(mesg.Torigin, m, p0, 2)
	}
}

func hcheck(m int) {
	reqd := false
	if m == Untagged {
		return
	}
	t := whichtext(m)
	if t == nil { /* possible in a half-built window */
		return
	}
	for i := 0; i < NL; i++ {
		l := &t.l[i]
		if l.textfn == nil || !flprepare(l) {
			/* BUG: don't need this if BUG below is fixed */
			// TODO(rsc): What BUG?
			continue
		}
		a := t.l[i].origin
		n := rcontig(&t.rasp, a, a+l.f.NumChars, true)
		if n < l.f.NumChars { /* text missing in middle of screen */
			a += n
		} else { /* text missing at end of screen? */
		Again:
			if l.f.LastLineFull {
				goto Checksel /* all's well */
			}
			a = t.l[i].origin + l.f.NumChars
			n = t.rasp.nrunes - a
			if n == 0 {
				goto Checksel
			}
			if n > mesg.TBLOCKSIZE {
				n = mesg.TBLOCKSIZE
			}
			n = rcontig(&t.rasp, a, a+n, true)
			if n > 0 {
				rp := rload(&t.rasp, a, a+n)
				nl := l.f.NumChars
				flinsert(l, rp, l.origin+nl)
				if nl == l.f.NumChars { /* made no progress */
					goto Checksel
				}
				goto Again
			}
		}
		if !reqd {
			n = rcontig(&t.rasp, a, a+mesg.TBLOCKSIZE, false)
			if n <= 0 {
				panic("hcheck request==0")
			}
			outTsls(mesg.Trequest, m, a, int(n))
			outTs(mesg.Tcheck, m)
			t.lock++ /* for the mesg.Trequest */
			t.lock++ /* for the mesg.Tcheck */
			reqd = true
		}
	Checksel:
		flsetselect(l, l.p0, l.p1)
	}
}

func flnewlyvisible(l *Flayer) {
	hcheck(l.text.tag)
}

func hsetsnarf(nc int) {
	display.SwitchCursor(&deadmouse)
	osnarf := make([]byte, nc)
	for i := range osnarf {
		osnarf[i] = byte(getch())
	}
	nsnarf := snarfswap(osnarf)
	if nsnarf != nil {
		if len(nsnarf) > mesg.SNARFSIZE {
			nsnarf = []byte("<snarf too long>")
		}
		snarflen = len(nsnarf)
		outTs(mesg.Tsetsnarf, len(nsnarf))
		if len(nsnarf) > 0 {
			if n, err := hostfd[1].Write(nsnarf); n != len(nsnarf) {
				panic("snarf write error: " + err.Error())
			}
		}
	} else {
		outTs(mesg.Tsetsnarf, 0)
	}
	display.SwitchCursor(cursor)
}

func hplumb(nc int) {
	s := make([]byte, nc)
	for i := range s {
		s[i] = byte(getch())
	}
	if plumbfd != nil {
		m := new(plumb.Message)
		if err := m.Recv(bytes.NewReader(s)); err == nil {
			m.Send(plumbfd)
		}
	}
}

// hgrow increases the size of the rasp to accomodate new input and
// sends points to host which will send back the data within those
// points and insert them into the rasp
func hgrow(m int, a int, new int, req int) {
	if new <= 0 {
		panic("hgrow")
	}
	t := whichtext(m)
	rresize(&t.rasp, a, 0, new)
	for i := 0; i < NL; i++ {
		l := &t.l[i]
		if l.textfn == nil {
			continue
		}
		o := l.origin
		b := a - o - rmissing(&t.rasp, o, a)
		if a < o {
			l.origin += new
		}
		if a < l.p0 {
			l.p0 += new
		}
		if a < l.p1 {
			l.p1 += new
		}
		/* must prevent b temporarily becoming unsigned */
		if req == 0 || a < o || (b > 0 && b > l.f.NumChars) || (l.f.NumChars == 0 && a-o > 0) {
			continue
		}
		if new > mesg.TBLOCKSIZE {
			new = mesg.TBLOCKSIZE
		}
		outTsls(mesg.Trequest, m, a, new)
		t.lock++
		req = 0
	}
}

func hdata1(t *Text, a int, rp []rune) int {
	for i := 0; i < NL; i++ {
		l := &t.l[i]
		if l.textfn == nil {
			continue
		}
		o := l.origin
		b := a - o - rmissing(&t.rasp, o, a)
		/* must prevent b temporarily becoming unsigned */
		if a < o || (b > 0 && b > l.f.NumChars) {
			continue
		}
		flinsert(l, rp, o+b)
	}
	rdata(&t.rasp, a, a+len(rp), rp)
	rclean(&t.rasp)
	return len(rp)
}

func hdata(m int, a int, s []byte) int {
	t := whichtext(m)
	if t.lock != 0 {
		t.lock--
	}
	if len(s) == 0 {
		return 0
	}
	r := []rune(string(s))
	return hdata1(t, a, r)
}

func hdatarune(m int, a int, rp []rune) int {
	t := whichtext(m)
	if t.lock != 0 {
		t.lock--
	}
	if len(rp) == 0 {
		return 0
	}
	return hdata1(t, a, rp)
}

func hcut(m, a, old int) {
	t := whichtext(m)
	if t.lock != 0 {
		t.lock--
	}
	for i := 0; i < NL; i++ {
		l := &t.l[i]
		if l.textfn == nil {
			continue
		}
		o := l.origin
		b := a - o - rmissing(&t.rasp, o, a)
		/* must prevent b temporarily becoming unsigned */
		if (b < 0 || b < l.f.NumChars) && a+old >= o {
			p := o
			if b >= 0 {
				p += b
			}
			fldelete(l, p, a+old-rmissing(&t.rasp, o, a+old))
		}
		if a+old < o {
			l.origin -= old
		} else if a <= o {
			l.origin = a
		}
		if a+old < l.p0 {
			l.p0 -= old
		} else if a <= l.p0 {
			l.p0 = a
		}
		if a+old < l.p1 {
			l.p1 -= old
		} else if a <= l.p1 {
			l.p1 = a
		}
	}
	rresize(&t.rasp, a, old, 0)
	rclean(&t.rasp)
}
