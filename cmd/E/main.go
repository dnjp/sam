package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func usage() {
	fmt.Printf("usage: E [+line] files...\n")
}

var opents = time.Now().Unix()

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	infiles := []string{}
	for _, f := range os.Args[1:] {
		nf, err := filepath.Abs(f)
		if err != nil {
			panic(err)
		}
		infiles = append(infiles, nf)
	}

	fmt.Printf("editing ")
	for _, f := range infiles {
		fmt.Printf("%s", f)
	}
	fmt.Printf("\n")

	err := exec.Command("B", os.Args[1:]...).Run()
	if err != nil {
		panic(err)
	}

	samfiles := []os.DirEntry{}
	files, err := os.ReadDir("/tmp")
	if err != nil {
		panic(err)
	}
	for _, f := range files {
		if strings.Contains(f.Name(), "sam.") {
			samfiles = append(samfiles, f)
		}
	}

	t := time.NewTicker(time.Second)
	for {
		<-t.C

		if len(samfiles) > 0 {
			written, err := findsamwrite("/tmp", samfiles, infiles)
			if err != nil {
				panic(err)
			}
			if written {
				return
			}
		}

		written, err := checkstat(infiles)
		if err != nil {
			panic(err)
		}
		if written {
			return
		}
	}
}

func checkstat(files []string) (bool, error) {
	for _, f := range files {
		nf, err := os.Open(f)
		if err != nil {
			return false, err
		}
		info, err := nf.Stat()
		if err != nil {
			return false, err
		}
		if info.ModTime().Unix() > opents {
			return true, nil
		}
	}
	return false, nil
}

func findsamwrite(base string, files []os.DirEntry, subs []string) (bool, error) {
	for _, f := range files {
		b, err := os.ReadFile(base + "/" + f.Name())
		if err != nil {
			return false, err
		}
		for _, s := range strings.Split(string(b), "\n") {
			for _, sub := range subs {
				writelog := strings.Split(s, "write=")
				if len(writelog) != 2 {
					continue
				}
				writets, err := strconv.Atoi(strings.TrimSpace(writelog[0]))
				if err != nil {
					return false, err
				}
				if int64(writets) < opents {
					continue
				}
				if strings.Contains(sub, writelog[1]) {
					return true, nil
				}
			}
		}
	}
	return false, nil
}
