package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dnjp/9fans/plumb"
	"github.com/dnjp/sam/kb"
	"github.com/dnjp/sam/mesg"
)

// #include "sam.h"
var h mesg.Header
var indata = make([]byte, mesg.DATASIZE)
var inp []uint8

var outdata [2*mesg.DATASIZE + 3]uint8 /* room for overflow message */
var outmsg = outdata[:0]               // messages completed but not sent
var outp []uint8                       // message being created in spare capacity of outmsg
var cmdpt Posn
var cmdptadv Posn
var snarfbuf Buffer
var waitack bool
var outbuffered bool
var tversion int
var journal_file *os.File

var hname = [26]string{
	mesg.Hversion:    "Hversion",
	mesg.Hbindname:   "Hbindname",
	mesg.Hcurrent:    "Hcurrent",
	mesg.Hnewname:    "Hnewname",
	mesg.Hmovname:    "Hmovname",
	mesg.Hgrow:       "Hgrow",
	mesg.Hcheck0:     "Hcheck0",
	mesg.Hcheck:      "Hcheck",
	mesg.Hunlock:     "Hunlock",
	mesg.Hdata:       "Hdata",
	mesg.Horigin:     "Horigin",
	mesg.Hunlockfile: "Hunlockfile",
	mesg.Hsetdot:     "Hsetdot",
	mesg.Hgrowdata:   "Hgrowdata",
	mesg.Hmoveto:     "Hmoveto",
	mesg.Hclean:      "Hclean",
	mesg.Hdirty:      "Hdirty",
	mesg.Hcut:        "Hcut",
	mesg.Hsetpat:     "Hsetpat",
	mesg.Hdelname:    "Hdelname",
	mesg.Hclose:      "Hclose",
	mesg.Hsetsnarf:   "Hsetsnarf",
	mesg.Hsnarflen:   "Hsnarflen",
	mesg.Hack:        "Hack",
	mesg.Hexit:       "Hexit",
	mesg.Hplumb:      "Hplumb",
}

var tname = [28]string{
	mesg.Tversion:      "Tversion",
	mesg.Tstartcmdfile: "Tstartcmdfile",
	mesg.Tcheck:        "Tcheck",
	mesg.Trequest:      "Trequest",
	mesg.Torigin:       "Torigin",
	mesg.Tstartfile:    "Tstartfile",
	mesg.Tworkfile:     "Tworkfile",
	mesg.Ttype:         "Ttype",
	mesg.Tcut:          "Tcut",
	mesg.Tpaste:        "Tpaste",
	mesg.Tsnarf:        "Tsnarf",
	mesg.Tstartnewfile: "Tstartnewfile",
	mesg.Twrite:        "Twrite",
	mesg.Tclose:        "Tclose",
	mesg.Tlook:         "Tlook",
	mesg.Tsearch:       "Tsearch",
	mesg.Tsend:         "Tsend",
	mesg.Tdclick:       "Tdclick",
	mesg.Tstartsnarf:   "Tstartsnarf",
	mesg.Tsetsnarf:     "Tsetsnarf",
	mesg.Tack:          "Tack",
	mesg.Texit:         "Texit",
	mesg.Tplumb:        "Tplumb",
	mesg.Ttclick:       "Ttclick",
	mesg.Tundo:         "Tundo",
	mesg.Tindent:       "Tindent",
	mesg.Tunindent:     "Tunindent",
	mesg.Tcomment:      "Tcomment",
}

/*
// #ifdef DEBUG

func journal(out int, s string) {
	if journal_file == nil {
		f, err := os.OpenFile("/tmp/sam.out", os.O_RDWR|os.O_CREATE, 0666)
		if err != nil {
			panic(err)
		}
		journal_file = f
	}
	var op string
	if out != 0 {
		op = "out: "
	} else {
		op = "in:  "
	}
	fmt.Fprintf(journal_file, "%s%s\n", op, s)
}

func journaln(out int, n int) {
	journal(out, fmt.Sprintf("%d", n))
}

func journalv(out int, v int64) {
	journal(out, fmt.Sprintf("%d", v))
}

// #else
// #define	// journal(a, b)
// #define journaln(a, b)
// #endif
*/

func journal(out int, s string) {}
func journaln(out, n int)       {}
func journalv(out, v int64)     {}

var rcvchar_nleft int = 0
var rcvchar_buf [64]uint8
var rcvchar_i int

func rcvchar() int {
	if rcvchar_nleft <= 0 {
		n, err := os.Stdin.Read(rcvchar_buf[:])
		if err != nil || n <= 0 {
			return -1
		}
		rcvchar_nleft = n
		rcvchar_i = 0
	}
	c := rcvchar_buf[rcvchar_i]
	rcvchar_nleft--
	rcvchar_i++
	return int(c)
}

var rcv_state int = 0
var rcv_count int = 0
var rcv_i int = 0

func rcv() bool {
	for c := rcvchar(); c >= 0; c = rcvchar() {
		switch rcv_state {
		case 0:
			h.Typ = mesg.Tmesg(c).Wire()
			rcv_state++

		case 1:
			h.Count0 = uint8(c)
			rcv_state++

		case 2:
			h.Count1 = uint8(c)
			rcv_count = int(h.Count0) | int(h.Count1)<<8
			if rcv_count > mesg.DATASIZE {
				panic_("count>mesg.DATASIZE")
			}
			indata = indata[:0]
			if rcv_count == 0 {
				rcv_count = 0
				rcv_state = 0
				return inmesg(mesg.Tmesg(h.Typ))
			}
			rcv_state++

		case 3:
			indata = append(indata, byte(c))
			if len(indata) == rcv_count {
				rcv_count = 0
				rcv_state = 0
				return inmesg(mesg.Tmesg(h.Typ))
			}
		}
	}
	return false
}

func whichfile(tag int) *File {
	for _, f := range file {
		if f.tag == tag {
			return f
		}
	}
	hiccough("")
	return nil
}

func inmesg(type_ mesg.Tmesg) bool {
	debug("inmesg %v %x\n", type_, indata)
	if type_ > mesg.TMAX {
		panic_("inmesg")
	}

	journal(0, tname[type_])

	inp = indata
	var buf [1025]rune
	var i int
	var m int
	var s int
	var l int
	var l1 int
	var v int64
	var f *File
	var p0 Posn
	var p1 Posn
	var p Posn
	var r Range
	var str *String
	debug("EV TYPE: %d %s", type_, tname[type_])
	switch type_ {
	case -1:
		panic_("rcv error")
		fallthrough

	default:
		fmt.Fprintf(os.Stderr, "unknown type %d\n", type_)
		panic_("rcv unknown")
		fallthrough

	case mesg.Tversion:
		tversion = inshort()
		journaln(0, tversion)

	case mesg.Tstartcmdfile:
		v = invlong() /* for 64-bit pointers */
		journalv(0, v)
		Strdupl(&genstr, samname)
		cmd = newfile()
		cmd.unread = false
		outTsv(mesg.Hbindname, cmd.tag, v)
		outTs(mesg.Hcurrent, cmd.tag)
		logsetname(cmd, &genstr)
		cmd.rasp = new(PosnList)
		cmd.mod = false
		if len(cmdstr.s) != 0 {
			loginsert(cmd, 0, cmdstr.s)
			Strdelete(&cmdstr, 0, Posn(len(cmdstr.s)))
		}
		fileupdate(cmd, false, true)
		outT0(mesg.Hunlock)
	/* go through whichfile to check the tag */

	case mesg.Tcheck:
		outTs(mesg.Hcheck, whichfile(inshort()).tag)

	case mesg.Trequest:
		f = whichfile(inshort())
		p0 = inlong()
		p1 = p0 + inshort()
		journaln(0, p0)
		journaln(0, p1-p0)
		if f.unread {
			panic_("mesg.Trequest: unread")
		}
		if p1 > f.b.nc {
			p1 = f.b.nc
		}
		if p0 > f.b.nc { /* can happen e.g. scrolling during command */
			p0 = f.b.nc
		}
		if p0 == p1 {
			i = 0
			r.p2 = p0
			r.p1 = r.p2
		} else {
			r = rdata(f.rasp, p0, p1-p0)
			i = r.p2 - r.p1
			bufread(&f.b, r.p1, buf[:i])
		}
		outTslS(mesg.Hdata, f.tag, r.p1, tmprstr(buf[:i]))
		if Aflag {
			ft, ok := kb.FindFiletype(Strtoc(&f.name))
			if ok {
				f.tabwidth = ft.Tabwidth
				outTsv(mesg.Htabwidth, f.tag, int64(f.tabwidth))
				f.tabexpand = ft.Tabexpand
				var te int64
				if ft.Tabexpand {
					te = 1
				}
				outTsv(mesg.Htabexpand, f.tag, te)
			}
		}

	case mesg.Torigin:
		s = inshort()
		l = inlong()
		l1 = inlong()
		journaln(0, l1)
		lookorigin(whichfile(s), l, l1)

	case mesg.Tstartfile:
		termlocked++
		f = whichfile(inshort())
		if f.rasp == nil { /* this might be a duplicate message */
			f.rasp = new(PosnList)
		}
		current(f)
		outTsv(mesg.Hbindname, f.tag, invlong()) /* for 64-bit pointers */
		outTs(mesg.Hcurrent, f.tag)
		journaln(0, f.tag)
		if f.unread {
			load(f)
		} else {
			if f.b.nc > 0 {
				rgrow(f.rasp, 0, f.b.nc)
				outTsll(mesg.Hgrow, f.tag, 0, f.b.nc)
			}
			outTs(mesg.Hcheck0, f.tag)
			moveto(f, f.dot.r)
		}

	case mesg.Tworkfile:
		i = inshort()
		f = whichfile(i)
		current(f)
		f.dot.r.p1 = inlong()
		f.dot.r.p2 = inlong()
		f.tdot = f.dot.r
		journaln(0, i)
		journaln(0, f.dot.r.p1)
		journaln(0, f.dot.r.p2)

	case mesg.Ttype:
		f = whichfile(inshort())
		p0 = inlong()
		journaln(0, p0)
		journal(0, (string)(inp))
		str = tmpcstr((string)(inp))
		i = len(str.s)
		debug("Ttype %s %d %q\n", f.name, p0, str)
		loginsert(f, p0, str.s)
		if fileupdate(f, false, false) {
			seq++
		}
		if f == cmd && p0 == f.b.nc-i && i > 0 && str.s[i-1] == '\n' {
			freetmpstr(str)
			termlocked++
			termcommand()
		} else {
			freetmpstr(str)
		} /* terminal knows this already */
		f.dot.r.p2 = p0 + i
		f.dot.r.p1 = f.dot.r.p2
		f.tdot = f.dot.r

	case mesg.Tcut:
		f = whichfile(inshort())
		p0 = inlong()
		p1 = inlong()
		journaln(0, p0)
		journaln(0, p1)
		logdelete(f, p0, p1)
		if fileupdate(f, false, false) {
			seq++
		}
		f.dot.r.p2 = p0
		f.dot.r.p1 = f.dot.r.p2
		f.tdot = f.dot.r /* terminal knows the value of dot already */

	case mesg.Tpaste:
		f = whichfile(inshort())
		p0 = inlong()
		journaln(0, p0)
		for l = 0; l < snarfbuf.nc; l += m {
			m = snarfbuf.nc - l
			if m > BLOCKSIZE {
				m = BLOCKSIZE
			}
			bufread(&snarfbuf, l, genbuf[:m])
			loginsert(f, p0, tmprstr(genbuf[:m]).s) // TODO(rsc): had ", m"
		}
		if fileupdate(f, false, true) {
			seq++
		}
		f.dot.r.p1 = p0
		f.dot.r.p2 = p0 + snarfbuf.nc
		f.tdot.p1 = -1 /* force telldot to tell (arguably a BUG) */
		telldot(f)
		outTs(mesg.Hunlockfile, f.tag)

	case mesg.Tsnarf:
		i = inshort()
		p0 = inlong()
		p1 = inlong()
		snarf(whichfile(i), p0, p1, &snarfbuf, 0)
		rp := make([]rune, snarfbuf.nc)
		bufread(&snarfbuf, 0, rp)
		m = snarfbuf.nc
		if m > mesg.SNARFSIZE {
			m = mesg.SNARFSIZE
			dprint("?warning: snarf buffer truncated\n")
		}
		c := []byte(string(rp)) // TODO(rsc)
		outTs(mesg.Hsetsnarf, len(c))
		os.Stdout.Write(c)

	case mesg.Tundo:
		f = whichfile(inshort())
		u_cmd(f, &Cmd{num: 1})

	case mesg.Tindent, mesg.Tunindent:
		indenting := type_ == mesg.Tindent
		f = whichfile(inshort())
		p0 = inlong()
		journaln(0, p0)
		var count int
		ft, ok := kb.FindFiletype(Strtoc(&f.name))
		if !ok {
			if f.tabwidth > 0 {
				ft.Tabwidth = f.tabwidth
			}
			if f.tabexpand {
				ft.Tabexpand = f.tabexpand
			}
		}
		for l = 0; l < snarfbuf.nc; l += m {
			m = snarfbuf.nc - l
			if m > BLOCKSIZE {
				m = BLOCKSIZE
			}
			bufread(&snarfbuf, l, genbuf[:m])
			rp, err := ft.IndentSelection(genbuf[:m], !indenting)
			if err != nil {
				panic(err)
			}
			count += len(rp)
			loginsert(f, p0, tmprstr(rp).s)
		}
		if fileupdate(f, false, true) {
			seq++
		}
		f.dot.r.p1 = p0
		f.dot.r.p2 = p0 + count
		f.tdot.p1 = -1 /* force telldot to tell (arguably a BUG) */
		outTs(mesg.Hunlockfile, f.tag)
		telldot(f)

	case mesg.Tcomment:
		f = whichfile(inshort())
		p0 = inlong()
		journaln(0, p0)
		var count int
		ft, _ := kb.FindFiletype(Strtoc(&f.name))
		if len(f.comment) != 0 {
			ft.Comment = string(f.comment)
		}
		for l = 0; l < snarfbuf.nc; l += m {
			m = snarfbuf.nc - l
			if m > BLOCKSIZE {
				m = BLOCKSIZE
			}
			bufread(&snarfbuf, l, genbuf[:m])
			rp, err := ft.CommentSelection(genbuf[:m])
			if err != nil {
				panic(err)
			}
			count += len(rp)
			loginsert(f, p0, tmprstr(rp).s)
		}
		if fileupdate(f, false, true) {
			seq++
		}
		f.dot.r.p1 = p0
		f.dot.r.p2 = p0 + count
		f.tdot.p1 = -1 /* force telldot to tell (arguably a BUG) */
		outTs(mesg.Hunlockfile, f.tag)
		telldot(f)

	case mesg.Tstartnewfile:
		v = invlong()
		Strdupl(&genstr, empty)
		f = newfile()
		f.rasp = new(PosnList)
		outTsv(mesg.Hbindname, f.tag, v)
		logsetname(f, &genstr)
		outTs(mesg.Hcurrent, f.tag)
		current(f)
		load(f)

	case mesg.Twrite:
		termlocked++
		i = inshort()
		journaln(0, i)
		f = whichfile(i)
		addr.r.p1 = 0
		addr.r.p2 = f.b.nc
		if len(f.name.s) == 0 {
			error_(Enoname)
		}
		Strduplstr(&genstr, &f.name)
		writef(f)
		logwrite(f.name)

	case mesg.Tclose:
		termlocked++
		i = inshort()
		journaln(0, i)
		f = whichfile(i)
		current(f)
		trytoclose(f)
		/* if trytoclose fails, will error out */
		delete(f)

	case mesg.Tlook:
		f = whichfile(inshort())
		termlocked++
		p0 = inlong()
		p1 = inlong()
		journaln(0, p0)
		journaln(0, p1)
		setgenstr(f, p0, p1)
		for l = 0; l < len(genstr.s); l++ {
			i := genstr.s[l]
			if strings.ContainsRune(".*+?(|)\\[]^$", i) {
				str = tmpcstr("\\")
				Strinsert(&genstr, str, l)
				l++
				freetmpstr(str)
			}
		}
		nextmatch(f, &genstr, p1, 1)
		moveto(f, sel.p[0])

	case mesg.Tsearch:
		termlocked++
		if curfile == nil {
			error_(Enofile)
		}
		if len(lastpat.s) == 0 {
			panic_("Tsearch")
		}
		nextmatch(curfile, &lastpat, curfile.dot.r.p2, 1)
		moveto(curfile, sel.p[0])

	case mesg.Tsend:
		termlocked++
		inshort() /* ignored */
		p0 = inlong()
		p1 = inlong()
		setgenstr(cmd, p0, p1)
		bufreset(&snarfbuf)
		bufinsert(&snarfbuf, Posn(0), genstr.s)
		outTl(mesg.Hsnarflen, len(genstr.s))
		if len(genstr.s) > 0 && genstr.s[len(genstr.s)-1] != '\n' {
			Straddc(&genstr, '\n')
		}
		loginsert(cmd, cmd.b.nc, genstr.s)
		fileupdate(cmd, false, true)
		cmd.dot.r.p2 = cmd.b.nc
		cmd.dot.r.p1 = cmd.dot.r.p2
		telldot(cmd)
		termcommand()

	case mesg.Tdclick:
		fallthrough
	case mesg.Ttclick:
		f = whichfile(inshort())
		p1 = inlong()
		stretchsel(f, p1, type_ == mesg.Ttclick)
		f.tdot.p2 = p1
		f.tdot.p1 = f.tdot.p2
		telldot(f)
		outTs(mesg.Hunlockfile, f.tag)

	case mesg.Tstartsnarf:
		if snarfbuf.nc <= 0 { /* nothing to export */
			outTs(mesg.Hsetsnarf, 0)
			break
		}
		m = snarfbuf.nc
		if m > mesg.SNARFSIZE {
			m = mesg.SNARFSIZE
			dprint("?warning: snarf buffer truncated\n")
		}
		rp := make([]rune, m)
		bufread(&snarfbuf, 0, rp)
		c := []byte(string(rp)) // TODO(rsc)
		outTs(mesg.Hsetsnarf, len(c))
		os.Stdout.Write(c)

	case mesg.Tsetsnarf:
		m = inshort()
		if m > mesg.SNARFSIZE {
			error_(Etoolong)
		}
		c := make([]byte, m)
		for i := 0; i < m; i++ {
			c[i] = byte(rcvchar())
		}
		str := []rune(string(c)) // TODO(rsc)
		bufreset(&snarfbuf)
		bufinsert(&snarfbuf, Posn(0), str)
		outT0(mesg.Hunlock)

	case mesg.Tack:
		waitack = false

	case mesg.Tplumb:
		f = whichfile(inshort())
		p0 = inlong()
		p1 = inlong()
		pm := new(plumb.Message)
		pm.Src = "sam"
		/* construct current directory */
		c := string(f.name.s)
		if len(c) > 0 && c[0] == '/' {
			pm.Dir = c
		} else {
			wd, _ := os.Getwd()
			pm.Dir = filepath.Join(wd, c)
		}
		if i := strings.LastIndex(pm.Dir, "/"); i >= 0 {
			pm.Dir = pm.Dir[:i]
		}
		pm.Type = "text"
		if p1 > p0 {
			pm.Attr = nil
		} else {
			p = p0
			for p0 > 0 {
				p0--
			}
			for p1 < f.b.nc {
				p1++
			}
			pm.Attr = &plumb.Attribute{Name: "click", Value: fmt.Sprint(p - p0)}
		}
		if p0 == p1 || p1-p0 >= BLOCKSIZE {
			// plumbfree(pm)
			break
		}
		setgenstr(f, p0, p1)
		pm.Data = []byte(string(genstr.s))
		var enc bytes.Buffer
		pm.Send(&enc)
		outTs(mesg.Hplumb, enc.Len())
		os.Stdout.Write(enc.Bytes())
		// free(enc)
		// plumbfree(pm)

	case mesg.Texit:
		os.Exit(0)
	}
	return true
}

func snarf(f *File, p1 Posn, p2 Posn, buf *Buffer, emptyok int) {
	if emptyok == 0 && p1 == p2 {
		return
	}
	bufreset(buf)
	/* Stage through genbuf to avoid compaction problems (vestigial) */
	if p2 > f.b.nc {
		fmt.Fprintf(os.Stderr, "bad snarf addr p1=%d p2=%d f->b.nc=%d\n", p1, p2, f.b.nc) /*ZZZ should never happen, can remove */
		p2 = f.b.nc
	}
	var n int
	for l := p1; l < p2; l += n {
		n = p2 - l
		if n > BLOCKSIZE {
			n = BLOCKSIZE
		}
		bufread(&f.b, l, genbuf[:n])
		bufinsert(buf, buf.nc, tmprstr(genbuf[:n]).s) // TODO was ,n
	}
}

func inshort() int {
	n := binary.LittleEndian.Uint16(inp)
	inp = inp[2:]
	return int(n)
}

func inlong() int {
	n := binary.LittleEndian.Uint32(inp)
	inp = inp[4:]
	return int(n)
}

func invlong() int64 {
	v := binary.LittleEndian.Uint64(inp)
	inp = inp[8:]
	return int64(v)
}

func setgenstr(f *File, p0 Posn, p1 Posn) {
	if p0 != p1 {
		if p1-p0 >= mesg.TBLOCKSIZE {
			error_(Etoolong)
		}
		Strinsure(&genstr, p1-p0)
		bufread(&f.b, p0, genbuf[:p1-p0])
		copy(genstr.s, genbuf[:])
	} else {
		if snarfbuf.nc == 0 {
			error_(Eempty)
		}
		if snarfbuf.nc > mesg.TBLOCKSIZE {
			error_(Etoolong)
		}
		bufread(&snarfbuf, Posn(0), genbuf[:snarfbuf.nc])
		Strinsure(&genstr, snarfbuf.nc)
		copy(genstr.s, genbuf[:])
	}
}

func outT0(type_ mesg.Hmesg) {
	outstart(type_)
	outsend()
}

func outTl(type_ mesg.Hmesg, l int) {
	outstart(type_)
	outlong(l)
	outsend()
}

func outTs(type_ mesg.Hmesg, s int) {
	outstart(type_)
	journaln(1, s)
	outshort(s)
	outsend()
}

func outS(s *String) {
	c := []byte(string(s.s)) // TODO(rsc)
	outcopy(c)
	journaln(1, len(c))
	// if len(c) > 99 { c = c[:99] }
	journal(1, string(c))
	// free(c)
}

func outTsS(type_ mesg.Hmesg, s1 int, s *String) {
	outstart(type_)
	outshort(s1)
	outS(s)
	outsend()
}

func outTslS(type_ mesg.Hmesg, s1 int, l1 Posn, s *String) {
	outstart(type_)
	outshort(s1)
	journaln(1, s1)
	outlong(l1)
	journaln(1, l1)
	outS(s)
	outsend()
}

func outTS(type_ mesg.Hmesg, s *String) {
	outstart(type_)
	outS(s)
	outsend()
}

func outTsllS(type_ mesg.Hmesg, s1 int, l1 Posn, l2 Posn, s *String) {
	outstart(type_)
	outshort(s1)
	outlong(l1)
	outlong(l2)
	journaln(1, l1)
	journaln(1, l2)
	outS(s)
	outsend()
}

func outTsll(type_ mesg.Hmesg, s int, l1 Posn, l2 Posn) {
	outstart(type_)
	outshort(s)
	outlong(l1)
	outlong(l2)
	journaln(1, l1)
	journaln(1, l2)
	outsend()
}

func outTsl(type_ mesg.Hmesg, s int, l Posn) {
	outstart(type_)
	outshort(s)
	outlong(l)
	journaln(1, l)
	outsend()
}

func outTsv(type_ mesg.Hmesg, s int, v int64) {
	outstart(type_)
	outshort(s)
	outvlong(v)
	journalv(1, v)
	outsend()
}

func outstart(typ mesg.Hmesg) {
	// journal(1, hname[typ])
	outp = outmsg[len(outmsg):len(outmsg)]
	outp = append(outp, byte(typ), 0, 0)
}

func outcopy(data []byte) {
	outp = append(outp, data...)
}

func outshort(s int) {
	outp = append(outp, byte(s), byte(s>>8))
}

func outlong(l int) {
	outp = append(outp, byte(l), byte(l>>8), byte(l>>16), byte(l>>24))
}

func outvlong(v int64) {
	outp = append(outp, byte(v), byte(v>>8), byte(v>>16), byte(v>>24),
		byte(v>>32), byte(v>>40), byte(v>>48), byte(v>>56))
}

func outsend() {
	if len(outp) >= cap(outmsg)-len(outmsg) {
		panic_("outsend")
	}
	outcount := len(outp)
	outcount -= 3
	outp[1] = byte(outcount)
	outp[2] = byte(outcount >> 8)
	outmsg = outmsg[:len(outmsg)+len(outp)]
	if !outbuffered {
		if nw, err := os.Stdout.Write(outmsg); err != nil || nw != len(outmsg) {
			rescue()
		}
		outmsg = outdata[:0]
		return
	}
}

func needoutflush() bool {
	return len(outmsg) >= mesg.DATASIZE
}

func outflush() {
	if len(outmsg) == 0 {
		return
	}
	outbuffered = false
	/* flow control */
	outT0(mesg.Hack)
	waitack = true
	for {
		if !rcv() {
			rescue()
			os.Exit(1)
		}
		if !waitack {
			break
		}
	}
	outmsg = outdata[:0]
	outbuffered = true
}
