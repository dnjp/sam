package main

import (
	"fmt"
	"os"
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
