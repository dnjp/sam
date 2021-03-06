package mesg

import "unicode/utf8"

/* VERSION 1 introduces plumbing
2 increases SNARFSIZE from 4096 to 32000
3 adds a triple click
*/
const VERSION = 3

const (
	TBLOCKSIZE = 512                           /* largest piece of text sent to terminal */
	DATASIZE   = (utf8.UTFMax*TBLOCKSIZE + 30) /* ... including protocol header stuff */
	SNARFSIZE  = 32000                         /* maximum length of exchanged snarf buffer, must fit in 15 bits */
)

/*
 * Messages originating at the terminal
 */
type Tmesg int

const (
	Tversion Tmesg = iota
	Tstartcmdfile
	Tcheck
	Trequest
	Torigin
	Tstartfile
	Tworkfile
	Ttype
	Tcut
	Tpaste
	Tsnarf
	Tstartnewfile
	Twrite
	Tclose
	Tlook
	Tsearch
	Tsend
	Tdclick
	Tstartsnarf
	Tsetsnarf
	Tack
	Texit
	Tplumb
	Ttclick
	Tundo
	Tindent
	Tunindent
	Tcomment
	Tlog
	Tsetdot
	TMAX
)

var tname = [TMAX]string{
	Tversion:      "Tversion",
	Tstartcmdfile: "Tstartcmdfile",
	Tcheck:        "Tcheck",
	Trequest:      "Trequest",
	Torigin:       "Torigin",
	Tstartfile:    "Tstartfile",
	Tworkfile:     "Tworkfile",
	Ttype:         "Ttype",
	Tcut:          "Tcut",
	Tpaste:        "Tpaste",
	Tsnarf:        "Tsnarf",
	Tstartnewfile: "Tstartnewfile",
	Twrite:        "Twrite",
	Tclose:        "Tclose",
	Tlook:         "Tlook",
	Tsearch:       "Tsearch",
	Tsend:         "Tsend",
	Tdclick:       "Tdclick",
	Tstartsnarf:   "Tstartsnarf",
	Tsetsnarf:     "Tsetsnarf",
	Tack:          "Tack",
	Texit:         "Texit",
	Tplumb:        "Tplumb",
	Ttclick:       "Ttclick",
	Tundo:         "Tundo",
	Tindent:       "Tindent",
	Tunindent:     "Tunindent",
	Tcomment:      "Tcomment",
	Tlog:          "Tlog",
	Tsetdot:       "Tsetdot",
}

func Tname(t Tmesg) string {
	return tname[t]
}

/*
 * Messages originating at the host
 */
type Hmesg int

const (
	Hversion Hmesg = iota
	Hbindname
	Hcurrent
	Hnewname
	Hmovname
	Hgrow
	Hcheck0
	Hcheck
	Hunlock
	Hdata
	Horigin
	Hunlockfile
	Hsetdot
	Hgrowdata
	Hmoveto
	Hclean
	Hdirty
	Hcut
	Hsetpat
	Hdelname
	Hclose
	Hsetsnarf
	Hsnarflen
	Hack
	Hexit
	Hplumb
	Htabwidth  // tab width
	Htabexpand // tab expand
	Hcomment   // comment style
	HMAX
)

var hname = [HMAX]string{
	Hversion:    "Hversion",
	Hbindname:   "Hbindname",
	Hcurrent:    "Hcurrent",
	Hnewname:    "Hnewname",
	Hmovname:    "Hmovname",
	Hgrow:       "Hgrow",
	Hcheck0:     "Hcheck0",
	Hcheck:      "Hcheck",
	Hunlock:     "Hunlock",
	Hdata:       "Hdata",
	Horigin:     "Horigin",
	Hunlockfile: "Hunlockfile",
	Hsetdot:     "Hsetdot",
	Hgrowdata:   "Hgrowdata",
	Hmoveto:     "Hmoveto",
	Hclean:      "Hclean",
	Hdirty:      "Hdirty",
	Hcut:        "Hcut",
	Hsetpat:     "Hsetpat",
	Hdelname:    "Hdelname",
	Hclose:      "Hclose",
	Hsetsnarf:   "Hsetsnarf",
	Hsnarflen:   "Hsnarflen",
	Hack:        "Hack",
	Hexit:       "Hexit",
	Hplumb:      "Hplumb",
}

func Hname(t Hmesg) string {
	return hname[t]
}

type Header struct {
	Typ    int
	Count0 uint8
	Count1 uint8
	Data   [1]uint8
}

func (h Hmesg) Wire() int {
	return int(h)
}

func (t Tmesg) Wire() int {
	return int(t)
}

/*
 * File transfer protocol schematic, a la Holzmann
 * #define N	6
 *
 * chan h = [4] of { mtype };
 * chan t = [4] of { mtype };
 *
 * mtype = {	Hgrow, Hdata,
 * 		Hcheck, Hcheck0,
 * 		Trequest, Tcheck,
 * 	};
 *
 * active proctype host()
 * {	byte n;
 *
 * 	do
 * 	:: n <  N -> n++; t!Hgrow
 * 	:: n == N -> n++; t!Hcheck0
 *
 * 	:: h?Trequest -> t!Hdata
 * 	:: h?Tcheck   -> t!Hcheck
 * 	od
 * }
 *
 * active proctype term()
 * {
 * 	do
 * 	:: t?Hgrow   -> h!Trequest
 * 	:: t?Hdata   -> skip
 * 	:: t?Hcheck0 -> h!Tcheck
 * 	:: t?Hcheck  ->
 * 		if
 * 		:: h!Trequest -> progress: h!Tcheck
 * 		:: break
 * 		fi
 * 	od;
 * 	printf("term exits\n")
 * }
 *
 * From: gerard@research.bell-labs.com
 * Date: Tue Jul 17 13:47:23 EDT 2001
 * To: rob@research.bell-labs.com
 *
 * spin -c 	(or -a) spec
 * pcc -DNP -o pan pan.c
 * pan -l
 *
 * proves that there are no non-progress cycles
 * (infinite executions *not* passing through
 * the statement marked with a label starting
 * with the prefix "progress")
 *
 */
