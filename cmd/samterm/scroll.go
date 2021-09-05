package main

import (
	"image"
	"time"

	"github.com/dnjp/9fans/draw"
	"github.com/dnjp/9fans/draw/frame"
	"github.com/dnjp/sam/mesg"
)

var (
	scrtmp  *draw.Image
	scrback *draw.Image
	// true if the frame was scrolled during text selection
	scrolled bool
	// true while scrolling during text selection
	scrolling bool
	// channel for receiving scrolling start events during text selection
	scrollstart = make(chan struct{})
	// true after signal sent on scrollstart
	scrollsent bool
	// direction of scroll: <0 is up, >0 is down
	scrdl int
	// p0 for the text selection while scrolling
	scrp0 int
	// p1 for the text selection while scrolling
	scrp1 int
	// pointer to either p0 or p1 depending on which direction
	// the frame is being scrolled while text is being selected
	scrpt *int
)

func scrtemps() {
	if scrtmp != nil {
		return
	}
	var h int
	if !screensize(nil, &h) {
		h = 2048
	}
	scrtmp, _ = display.AllocImage(image.Rect(0, 0, 32, h), screen.Pix, false, 0)
	scrback, _ = display.AllocImage(image.Rect(0, 0, 32, h), screen.Pix, false, 0)
	if scrtmp == nil || scrback == nil {
		panic("scrtemps")
	}
}

func scrpos(r image.Rectangle, p0 int, p1 int, tot int) image.Rectangle {
	q := r
	h := q.Max.Y - q.Min.Y
	if tot == 0 {
		return q
	}
	if tot > 1024*1024 {
		tot >>= 10
		p0 >>= 10
		p1 >>= 10
	}
	if p0 > 0 {
		q.Min.Y += h * p0 / tot
	}
	if p1 < tot {
		q.Max.Y -= h * (tot - p1) / tot
	}
	if q.Max.Y < q.Min.Y+2 {
		if q.Min.Y+2 <= r.Max.Y {
			q.Max.Y = q.Min.Y + 2
		} else {
			q.Min.Y = q.Max.Y - 2
		}
	}
	return q
}

func scrmark(l *Flayer, r image.Rectangle) {
	r.Max.X--
	if draw.RectClip(&r, l.scroll) {
		l.f.B.Draw(r, l.f.Cols[frame.HIGH], nil, draw.ZP)
	}
}

func scrunmark(l *Flayer, r image.Rectangle) {
	if draw.RectClip(&r, l.scroll) {
		l.f.B.Draw(r, scrback, nil, image.Pt(0, r.Min.Y-l.scroll.Min.Y))
	}
}

func scrdraw(l *Flayer, tot int) {
	scrtemps()
	if l.f.B == nil {
		panic("scrdraw")
	}
	r := l.scroll
	r1 := r
	var b *draw.Image
	if l.visible == All {
		b = scrtmp
		r1.Min.X = 0
		r1.Max.X = r.Dx()
	} else {
		b = l.f.B
	}
	r2 := scrpos(r1, l.origin, l.origin+l.f.NumChars, tot)
	if r2 != l.lastsr {
		l.lastsr = r2
		b.Draw(r1, l.f.Cols[frame.BORD], nil, draw.ZP)
		b.Draw(r2, l.f.Cols[frame.BACK], nil, r2.Min)
		r2 = r1
		r2.Min.X = r2.Max.X - 1
		b.Draw(r2, l.f.Cols[frame.BORD], nil, draw.ZP)
		if b != l.f.B {
			l.f.B.Draw(r, b, nil, r1.Min)
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func scrorigin(l *Flayer, but int, p0 int) {
	t := l.text

	if t.tag == Untagged {
		return
	}

	// 1=up, 3=down
	switch but {
	case 1:
		outTsll(mesg.Torigin, t.tag, l.origin, p0)
	case 2:
		outTsll(mesg.Torigin, t.tag, p0, 1)
	case 3:
		horigin(t.tag, p0)
	}
}

func scroll(l *Flayer, but int) {
	up := but == 1 || but == 4
	down := but == 3 || but == 5
	exact := but == 2
	in := false
	tot := scrtotal(l)
	s := l.scroll
	x := s.Min.X + FLSCROLLWID(l)/2
	scr := scrpos(l.scroll, l.origin, l.origin+l.f.NumChars, tot)
	r := scr
	y := scr.Min.Y
	my := mousep.Point.Y
	scrback.Draw(image.Rect(0, 0, l.scroll.Dx(), l.scroll.Dy()), l.f.B, nil, l.scroll.Min)
	var p0 int

	if l.visible == None {
		return
	}

	for {
		oin := in
		in = abs(x-mousep.Point.X) <= FLSCROLLWID(l)/2
		if oin && !in {
			scrunmark(l, r)
		}
		if but > 3 || in {
			scrmark(l, r)
			oy := y
			my = mousep.Point.Y
			if my < s.Min.Y {
				my = s.Min.Y
			}
			if my >= s.Max.Y {
				my = s.Max.Y
			}
			if in && mousep.Point != image.Pt(x, my) {
				display.MoveCursor(image.Pt(x, my))
			}
			switch {
			case up:
				p0 = l.origin - l.f.CharOf(image.Pt(s.Max.X, my))
				rt := scrpos(l.scroll, p0, p0+l.f.NumChars, tot)
				y = rt.Min.Y
			case exact:
				y = my
				if y > s.Max.Y-2 {
					y = s.Max.Y - 2
				}
			case down:
				p0 = l.origin + l.f.CharOf(image.Pt(s.Max.X, my))
				rt := scrpos(l.scroll, p0, p0+l.f.NumChars, tot)
				y = rt.Min.Y
			}
			if y != oy {
				scrunmark(l, r)
				r = scr.Add(image.Pt(0, y-scr.Min.Y))
				scrmark(l, r)
			}
		}
		if but > 3 || button(but) == 0 {
			break
		}
	}

	if but > 3 || in {
		h := s.Max.Y - s.Min.Y
		scrunmark(l, r)
		p0 = 0
		switch {
		case up:
			but = 1
			if !in {
				p0 = 2
			} else {
				p0 = int(my-s.Min.Y)/l.f.Font.Height + 1
			}
		case exact:
			if tot > 1024*1024 {
				p0 = ((tot >> 10) * (y - s.Min.Y) / h) << 10
			} else {
				p0 = tot * (y - s.Min.Y) / h
			}
		case down:
			but = 3
			if !in {
				p0 = l.text.NextLine(l.origin)
			} else {
				p0 = l.origin + l.f.CharOf(image.Pt(s.Max.X, my))
				if p0 > tot {
					p0 = tot
				}
			}
		}
		scrorigin(l, but, p0)
	}
}

func scrsleep(dt int) {
	timer := time.NewTimer(time.Duration(dt) * time.Millisecond)
	for {
		select {
		case <-timer.C:
			return
		case <-mousectl.C:
			timer.Stop()
			return
		}
	}
}

func scrollf(f *frame.Frame, dl int) {
	scrolled = true
	scrolling = true
	scrdl = dl
	if !scrollsent {
		scrollstart <- struct{}{}
		scrollsent = true
	}
	if scrp0 <= 0 && scrdl > 0 {
		// scrolling down
		scrp0 = f.P0 + which.origin
		scrpt = &scrp1
	}
	if scrp1 <= 0 && scrdl < 0 {
		// scrolling up
		scrp1 = f.P1 + which.origin
		scrpt = &scrp0
	}
	if f != &which.f {
		panic("wrong frame for scroll")
	}
	if scrdl == 0 {
		scrsleep(100)
		return
	}
	scroll(which, func() int {
		if scrdl < 0 {
			return 4
		} else {
			return 5
		}
	}())
	mousectl.Read()
}
