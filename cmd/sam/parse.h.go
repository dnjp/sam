package main

type Addr struct {
	type_ rune
	are   *String
	left  *Addr
	num   Posn
	next  *Addr
}

// #define	are	g.re
// #define	left	g.aleft

type Cmd struct {
	addr  *Addr
	re    *String
	ccmd  *Cmd
	ctext *String
	caddr *Addr
	next  *Cmd
	num   int
	flag  bool
	cmdc  string
}

// #define	ccmd	g.cmd
// #define	ctext	g.text
// #define	caddr	g.addr

type Cmdtab struct {
	cmdc    string
	text    bool
	regexp  bool
	addr    bool
	defcmd  string
	defaddr Defaddr
	count   uint8
	token   string
	fn      func(*File, *Cmd) bool
}

/* extern var cmdtab [unknown]Cmdtab */ /* default addresses */

type Defaddr int

const (
	aNo Defaddr = iota
	aDot
	aAll
)
