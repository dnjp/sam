# Message Lifecycle

## Keyboard
- [waitforio](../cmd/samterm/io.go:85)
- [ktype](../cmd/samterm/main.go:577)
- [flushtyping](../cmd/samterm/main.go:483): 
  - sets hostlock and selection (`outTsll(mesg.Tworkfile, ...`) if rasp is full and the line is complete (\n). The lock is removed by receiving a `mesg.Hunlock` message.
  - sets `typeesc` which is used for selecting and/or cuting the selected text when ESC is pressed
  - sets `modified`
  - [outTslS](../cmd/samterm/mesg.go:457): `outTslS(mesg.Ttype, t.tag, typestart, rp)`
    - [outstart(typ)](../cmd/samterm/mesg.go:473): sets type in outdata
    - [outshort(s1)](../cmd/samterm/mesg.go:484): sets tag ID in outdata
    - [outlong(l1)](../cmd/samterm/mesg.go:489): sets start address in oudata
    - [outlong(l1)](../cmd/samterm/mesg.go:489): sets start address in oudata
    - [outrunes(s)](../cmd/samterm/mesg.go:478): sets runes entered in outdata
    - [outsend()](../cmd/samterm/mesg.go:478): writes the oudata to host file descriptor
- [cut](../cmd/samterm/main.go:330)
  - [snarf](../cmd/samterm/main.go:322): sets `snarflen` and sends snarf message if `save` is set
    - [`outTsll(mesg.Tsnarf...`](../cmd/samterm/mesg.go:429): reads the selection into sam's file buffer
  - [`outTsll(mesg.Tcut...`](cmd/samterm/mesg.go:429): deletes selection from sam's file buffer
  - `t.lock++`: sets text lock which is unlocked by a `Hcheck`, `Hunlockfile`, `Hdata`, or `Hcut` message