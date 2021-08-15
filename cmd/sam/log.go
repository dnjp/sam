package main

import (
	"fmt"
	"os"
	"time"
)

var tmpfile *os.File

func lognew() error {
	f, err := os.CreateTemp("/tmp", "sam.")
	if err != nil {
		return err
	}
	tmpfile = f
	tmpfile.Write([]byte(fmt.Sprintf("%d pid=%d\n", time.Now().Unix(), os.Getpid())))
	return nil
}

func logwrite(filename String) {
	logf := fmt.Sprintf("%d write=%s\n", time.Now().Unix(), Strtoc(&filename))
	tmpfile.Write([]byte(logf))
}

func logclose() {
	os.Remove(tmpfile.Name())
}
