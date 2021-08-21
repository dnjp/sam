package main

import (
	"image"

	"github.com/dnjp/9fans/draw/frame"
)

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
	f         frame.Frame
	origin    int
	p0        int
	p1        int
	click     uint32
	textfn    func(*Flayer, int) []rune
	text      *Text
	entire    image.Rectangle
	scroll    image.Rectangle
	lastsr    image.Rectangle
	visible   Vis
	tabexpand bool
}

func FLMARGIN(l *Flayer) int    { return flscale(l, 2) }
func FLSCROLLWID(l *Flayer) int { return flscale(l, 12) }
func FLGAP(l *Flayer) int       { return flscale(l, 4) }
