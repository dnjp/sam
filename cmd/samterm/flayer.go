package main

import (
	"image"
	"math"

	"github.com/dnjp/9fans/draw"
	"github.com/dnjp/9fans/draw/frame"
	"github.com/dnjp/sam/mesg"
)

var llist []*Flayer /* front to back */
var lDrect image.Rectangle

var maincols [frame.NCOL]*draw.Image
var cmdcols [frame.NCOL]*draw.Image

var clickcount int
var clickpt image.Point = image.Pt(-10, -10)

type Vis int

const (
	None Vis = 0 + iota
	Some
	All
)

const (
	Clicktime = 500 // millisecond
)

type Flayer struct {
	f       frame.Frame
	origin  int
	p0      int
	p1      int
	click   uint32
	textfn  func(*Flayer, int) []rune
	text    *Text
	entire  image.Rectangle
	scroll  image.Rectangle
	lastsr  image.Rectangle
	visible Vis
}

func FLMARGIN(l *Flayer) int    { return l.Scale(2) }
func FLSCROLLWID(l *Flayer) int { return l.Scale(12) }
func FLGAP(l *Flayer) int       { return l.Scale(4) }

func FlayerStart(r image.Rectangle) {
	lDrect = r

	/* Main text is yellowish */
	maincols[frame.BACK], _ = display.AllocImage(image.Rect(0, 0, 1, 1), screen.Pix, true, 0xFAFAF9FF)
	maincols[frame.HIGH], _ = display.AllocImage(image.Rect(0, 0, 1, 1), screen.Pix, true, 0xEBE7E7FF)
	maincols[frame.BORD], _ = display.AllocImage(image.Rect(0, 0, 2, 2), screen.Pix, true, 0xEBE7E7FF)
	maincols[frame.TEXT] = display.Black
	maincols[frame.HTEXT] = display.Black

	/* Command text is blueish */
	cmdcols[frame.BACK] = display.White
	cmdcols[frame.HIGH], _ = display.AllocImage(image.Rect(0, 0, 1, 1), screen.Pix, true, 0xBCB8B8FF)
	cmdcols[frame.BORD], _ = display.AllocImage(image.Rect(0, 0, 2, 2), screen.Pix, true, 0xBCB8B8FF)
	cmdcols[frame.TEXT] = display.Black
	cmdcols[frame.HTEXT] = display.Black
}

func (l *Flayer) New(fn func(*Flayer, int) []rune, text *Text) {
	l.textfn = fn
	l.text = text
	l.lastsr = draw.ZR
	InsertFlayer(l)
}

func (l *Flayer) Rect(r image.Rectangle) image.Rectangle {
	draw.RectClip(&r, lDrect)
	l.entire = r
	l.scroll = r.Inset(FLMARGIN(l))
	l.scroll.Max.X = r.Min.X + FLMARGIN(l) + FLSCROLLWID(l) + (FLGAP(l) - FLMARGIN(l))
	r.Min.X = l.scroll.Max.X
	return r
}

func (l *Flayer) Init(r image.Rectangle, ft *draw.Font, cols []*draw.Image) {
	DeleteFlayer(l)
	InsertFlayer(l)
	l.visible = All
	l.p1 = 0
	l.p0 = l.p1
	l.origin = l.p0
	l.f.Display = display // for FLMARGIN
	l.f.Scroll = scrollf
	l.f.Init(l.Rect(r).Inset(FLMARGIN(l)), ft, screen, cols)
	l.f.MaxTab = maxtab * ft.StringWidth("0")
	l.text.tabexpand = false
	newvisibilities(true)
	screen.Draw(l.entire, l.f.Cols[frame.BACK], nil, draw.ZP)
	scrdraw(l, 0)
	l.Border(false)
}

func (l *Flayer) Close() {
	if l.visible == All {
		screen.Draw(l.entire, display.White, nil, draw.ZP)
	} else if l.visible == Some {
		if l.f.B == nil {
			l.f.B, _ = display.AllocImage(l.entire, screen.Pix, false, draw.NoFill)
		}
		if l.f.B != nil {
			l.f.B.Draw(l.entire, display.White, nil, draw.ZP)
			l.Refresh(l.entire, 0)
		}
	}
	l.f.Clear(true)
	DeleteFlayer(l)
	if l.f.B != nil && l.visible != All {
		l.f.B.Free()
	}
	l.textfn = nil
	newvisibilities(true)
}

func (l *Flayer) Border(wide bool) {
	if l.Prepare() {
		l.f.B.Border(l.entire, FLMARGIN(l), l.f.Cols[frame.BACK], draw.ZP)
		w := 1
		if wide {
			w = FLMARGIN(l)
		}
		l.f.B.Border(l.entire, w, l.f.Cols[frame.BORD], draw.ZP)
		if l.visible == Some {
			l.Refresh(l.entire, 0)
		}
	}
}

func WhichFlayer(p image.Point) *Flayer {
	if p.X == 0 && p.Y == 0 {
		if len(llist) > 0 {
			return llist[0]
		}
		return nil
	}
	for _, l := range llist {
		if p.In(l.entire) {
			return l
		}
	}
	return nil
}

func (l *Flayer) UpFront() {
	v := l.visible
	DeleteFlayer(l)
	InsertFlayer(l)
	if v != All {
		newvisibilities(false)
	}
}

func newvisibilities(redraw bool) {
	/* if redraw false, we know it's a UpFront, and needn't
	 * redraw anyone becoming partially covered */
	for _, l := range llist {
		l.lastsr = draw.ZR /* make sure scroll bar gets redrawn */
		ov := l.visible
		l.visible = l.Visibility()
		V := func(a, b Vis) int { return int(a)<<2 | int(b) }
		switch V(ov, l.visible) {
		case V(Some, None):
			if l.f.B != nil {
				l.f.B.Free()
			}
			fallthrough
		case V(All, None),
			V(All, Some):
			l.f.B = nil
			l.f.Clear(false)

		case V(Some, Some),
			V(None, Some):
			if ov == None || (l.f.B == nil && redraw) {
				l.Prepare()
			}
			if l.f.B != nil && redraw {
				l.Refresh(l.entire, 0)
				l.f.B.Free()
				l.f.B = nil
				l.f.Clear(false)
			}
			fallthrough
		case V(None, None),
			V(All, All):
			break

		case V(Some, All):
			if l.f.B != nil {
				screen.Draw(l.entire, l.f.B, nil, l.entire.Min)
				l.f.B.Free()
				l.f.B = screen
				break
			}
			fallthrough
		case V(None, All):
			l.Prepare()
		}
		if ov == None && l.visible != None {
			Newlyvisible(l)
		}
	}
}

func InsertFlayer(l *Flayer) {
	llist = append(llist, nil)
	copy(llist[1:], llist)
	llist[0] = l
}

func DeleteFlayer(l *Flayer) {
	for i := range llist {
		if llist[i] == l {
			copy(llist[i:], llist[i+1:])
			llist = llist[:len(llist)-1]
			return
		}
	}
	panic("DeleteFlayer")
}

func (l *Flayer) Insert(rp []rune, p0 int) {
	if l.Prepare() {
		l.f.Insert(rp, p0-l.origin)
		scrdraw(l, scrtotal(l))
		if l.visible == Some {
			l.Refresh(l.entire, 0)
		}
	}
}

func (l *Flayer) Delete(p0 int, p1 int) {
	if l.Prepare() {
		p0 -= l.origin
		if p0 < 0 {
			p0 = 0
		}
		p1 -= l.origin
		if p1 < 0 {
			p1 = 0
		}
		l.f.Delete(p0, p1)
		scrdraw(l, scrtotal(l))
		if l.visible == Some {
			l.Refresh(l.entire, 0)
		}
	}
}

func (l *Flayer) Select() int {
	if l.visible != All {
		l.UpFront()
	}
	dt := int(mousep.Msec - l.click)
	dx := int(math.Abs(float64(mousep.Point.X - clickpt.X)))
	dy := int(math.Abs(float64(mousep.Point.Y - clickpt.Y)))
	l.click = mousep.Msec
	clickpt = mousep.Point

	if dx < 3 && dy < 3 && dt < Clicktime && clickcount < 3 {
		clickcount++
		return clickcount
	}

	clickcount = 0
	scrolled = false
	scrolling = false
	done := make(chan struct{})

	go func(done chan struct{}) {
		l.f.Select(mousectl)
		l.p0 = l.f.P0 + l.origin
		l.p1 = l.f.P1 + l.origin
		if scrolled {
			l.SetSelect(scrp0, scrp1)
			outTsll(mesg.Tsetdot, l.text.tag, l.f.P0, l.f.P1)
		}
		// reset scrolling state
		scrolled = false
		scrolling = false
		scrollsent = false
		scrp0 = 0
		scrp1 = 0
		scrpt = nil
		done <- struct{}{}
	}(done)

	select {
	case <-done:
	case <-scrollstart:
	}
	return 0
}

func (l *Flayer) SetSelect(p0 int, p1 int) {
	var fp0, fp1 int
	var ticked bool

	if l.visible == None || !l.Prepare() {
		l.p0 = p0
		l.p1 = p1
		return
	}

	l.p0 = p0
	l.p1 = p1
	if scrolling {
		fp0 = l.p0
		fp1 = l.p1
		ticked = l.f.Ticked
		goto Refresh
	}

	fp0, fp1, ticked = l.flfp0p1()

	if fp0 == l.f.P0 && fp1 == l.f.P1 {
		if l.f.Ticked != ticked {
			l.f.Tick(l.f.PointOf(fp0), ticked)
		}
		return
	}

	if fp1 <= l.f.P0 || fp0 >= l.f.P1 || l.f.P0 == l.f.P1 || fp0 == fp1 {
		/* no overlap or trivial repainting */
		l.f.Drawsel(l.f.PointOf(l.f.P0), l.f.P0, l.f.P1, false)
		if fp0 != fp1 || ticked {
			l.f.Drawsel(l.f.PointOf(fp0), fp0, fp1, true)
		}
		goto Refresh
	}
	/* the current selection and the desired selection overlap and are both non-empty */
	if fp0 < l.f.P0 {
		/* extend selection backwards */
		l.f.Drawsel(l.f.PointOf(fp0), fp0, l.f.P0, true)
	} else if fp0 > l.f.P0 {
		/* trim first part of selection */
		l.f.Drawsel(l.f.PointOf(l.f.P0), l.f.P0, fp0, false)
	}
	if fp1 > l.f.P1 {
		/* extend selection forwards */
		l.f.Drawsel(l.f.PointOf(l.f.P1), l.f.P1, fp1, true)
	} else if fp1 < l.f.P1 {
		/* trim last part of selection */
		l.f.Drawsel(l.f.PointOf(fp1), fp1, l.f.P1, false)
	}

Refresh:
	l.f.P0 = fp0
	l.f.P1 = fp1

	if scrolling && scrpt != nil {
		*scrpt = l.f.CharOf(mousectl.Point)
		if scrpt == &scrp1 {
			l.f.P1 = *scrpt + l.origin
		}
		if scrpt == &scrp0 {
			l.f.P0 = *scrpt + l.origin
		}
	}

	if l.visible == Some {
		l.Refresh(l.entire, 0)
	}
}

func (l *Flayer) flfp0p1() (p0 int, p1 int, ticked bool) {
	p0 = l.p0 - l.origin
	p1 = l.p1 - l.origin
	ticked = p0 == p1

	if p0 < 0 {
		ticked = false
		p0 = 0
	}
	if p1 < 0 {
		p1 = 0
	}
	if p0 > l.f.NumChars {
		p0 = l.f.NumChars
	}
	if p1 > l.f.NumChars {
		ticked = false
		p1 = l.f.NumChars
	}

	return p0, p1, ticked
}

func rscale(r image.Rectangle, old image.Point, new image.Point) image.Rectangle {
	r.Min.X = r.Min.X * new.X / old.X
	r.Min.Y = r.Min.Y * new.Y / old.Y
	r.Max.X = r.Max.X * new.X / old.X
	r.Max.Y = r.Max.Y * new.Y / old.Y
	return r
}

func ResizeFlayer(dr image.Rectangle) {
	olDrect := lDrect
	lDrect = dr
	move := false
	/* no moving on rio; must repaint */
	if false && dr.Dx() == olDrect.Dx() && dr.Dy() == olDrect.Dy() {
		move = true
	} else {
		screen.Draw(lDrect, display.White, nil, draw.ZP)
	}
	for _, l := range llist {
		l.lastsr = draw.ZR
		f := &l.f
		var r image.Rectangle
		if move {
			r = l.entire.Sub(olDrect.Min).Add(dr.Min)
		} else {
			r = rscale(l.entire.Sub(olDrect.Min), olDrect.Max.Sub(olDrect.Min), dr.Max.Sub(dr.Min)).Add(dr.Min)
			if l.visible == Some && f.B != nil {
				f.B.Free()
				f.Clear(false)
			}
			f.B = nil
			if l.visible != None {
				f.Clear(false)
			}
		}
		if !draw.RectClip(&r, dr) {
			panic("ResizeFlayer")
		}
		if r.Max.X-r.Min.X < 100 {
			r.Min.X = dr.Min.X
		}
		if r.Max.X-r.Min.X < 100 {
			r.Max.X = dr.Max.X
		}
		if r.Max.Y-r.Min.Y < 2*FLMARGIN(l)+f.Font.Height {
			r.Min.Y = dr.Min.Y
		}
		if r.Max.Y-r.Min.Y < 2*FLMARGIN(l)+f.Font.Height {
			r.Max.Y = dr.Max.Y
		}
		if !move {
			l.visible = None
		}
		f.SetRects(l.Rect(r).Inset(FLMARGIN(l)), f.B)
		if !move && f.B != nil {
			scrdraw(l, scrtotal(l))
		}
	}
	newvisibilities(true)
}

func (l *Flayer) Prepare() bool {
	if l.visible == None {
		return false
	}
	f := &l.f
	if f.B == nil {
		if l.visible == All {
			f.B = screen
		} else {
			f.B, _ = display.AllocImage(l.entire, screen.Pix, false, 0)
			if f.B == nil {
				return false
			}
		}
		f.B.Draw(l.entire, f.Cols[frame.BACK], nil, draw.ZP)
		w := 1
		if l == llist[0] {
			w = FLMARGIN(l)
		}
		f.B.Border(l.entire, w, f.Cols[frame.BORD], draw.ZP)
		n := f.NumChars
		f.Init(f.Entire, f.Font, f.B, nil)
		f.MaxTab = maxtab * f.Font.StringWidth("0")
		rp := l.textfn(l, n)
		f.Insert(rp, 0)
		f.Drawsel(f.PointOf(f.P0), f.P0, f.P1, false)
		var ticked bool
		f.P0, f.P1, ticked = l.flfp0p1()
		if f.P0 != f.P1 || ticked {
			f.Drawsel(f.PointOf(f.P0), f.P0, f.P1, true)
		}
		l.lastsr = draw.ZR
		scrdraw(l, scrtotal(l))
	}
	return true
}

var somevis, someinvis, justvis bool

func (l *Flayer) Visibility() Vis {
	someinvis = false
	somevis = someinvis
	justvis = true
	l.Refresh(l.entire, 0)
	justvis = false
	if !somevis {
		return None
	}
	if !someinvis {
		return All
	}
	return Some
}

func (l *Flayer) Refresh(r image.Rectangle, i int) {
Top:
	t := llist[i]
	i++
	if t == l {
		if !justvis {
			screen.Draw(r, l.f.B, nil, r.Min)
		}
		somevis = true
	} else {
		if !draw.RectXRect(t.entire, r) {
			goto Top /* avoid stacking unnecessarily */
		}
		var s image.Rectangle
		if t.entire.Min.X > r.Min.X {
			s = r
			s.Max.X = t.entire.Min.X
			l.Refresh(s, i)
			r.Min.X = t.entire.Min.X
		}
		if t.entire.Min.Y > r.Min.Y {
			s = r
			s.Max.Y = t.entire.Min.Y
			l.Refresh(s, i)
			r.Min.Y = t.entire.Min.Y
		}
		if t.entire.Max.X < r.Max.X {
			s = r
			s.Min.X = t.entire.Max.X
			l.Refresh(s, i)
			r.Max.X = t.entire.Max.X
		}
		if t.entire.Max.Y < r.Max.Y {
			s = r
			s.Min.Y = t.entire.Max.Y
			l.Refresh(s, i)
			r.Max.Y = t.entire.Max.Y
		}
		/* remaining piece of r is blocked by t; forget about it */
		someinvis = true
	}
}

func (l *Flayer) Scale(n int) int {
	if l == nil {
		return n
	}
	return l.f.Display.ScaleSize(n)
}

func FlayerInList(l *Flayer) bool {
	for _, fl := range llist {
		if l == fl {
			return true
		}
	}
	return false
}
