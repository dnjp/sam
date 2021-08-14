package main

import (
	"testing"
)

func TestRaspload(t *testing.T) {
	f := fileopen()
	raspload(f)
	if growpos != 0 {
		t.Fatal()
	}
	if f.rasp != nil {
		t.Fatal()
	}

	f.rasp = &PosnList{}
	raspload(f)
	if len(*f.rasp) != 0 {
		t.Fatal()
	}

	f.b.nc = 1
	raspload(f)
	if len(*f.rasp) != 1 {
		t.Fatal()
	}
	if (*f.rasp)[0] != 1 {
		t.Fatal()
	}
}

func TestRaspstart(t *testing.T) {
	f := fileopen()
	f.rasp = &PosnList{}
	raspstart(f)
	if grown != 0 {
		t.Fatal()
	}
	if shrunk != 0 {
		t.Fatal()
	}
	if !outbuffered {
		t.Fatal()
	}
}

func TestRaspdone(t *testing.T) {
	f := fileopen()
	f.rasp = &PosnList{}
	raspdone(f, false)
	if outbuffered {
		t.Fatal()
	}
	// TODO: test different f.dot.r values
	// TODO: test different f.mark values
	// TODO: test grown != 0
	//   TODO: test outTsll effects
	// TODO: test shrunk != 0
	//   TODO: test outTsll effects
	// TODO: test toterm=true
	//   TODO: test outTs effects
	// TODO: test setting cmd=file
}

// func TestRaspflush(t *testing.T) {
// 	// overflowsz := 2*DATASIZE + 3
// 	f := fileopen()
// 	f.rasp = &PosnList{}
// 	outdata[0] = uint8(1)
// 	outdata[1] = uint8(2)
// 	outdata[2] = uint8(3)
// 	outmsg = outdata[:1]
// 	t.Logf("outp:%d capoutmsg:%d, lenoutmsg:%d\n", len(outp), cap(outmsg), len(outmsg))
// 	raspflush(f)
// 	// if outbuffered {
// 	// t.Fatal()
// 	// }
// 	// if len(outdata) != overflowsz {
// 	// t.Fatal()
// 	// }
// 	// if len(outmsg) != 0 {
// 	// t.Fatal()
// 	// }
// 	// if !waitack {
// 	// t.Fatal()
// 	// }
// }
