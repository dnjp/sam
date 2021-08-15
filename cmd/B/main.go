package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var validline = regexp.MustCompile(`^\+[0-9]*`)

func usage() {
	fmt.Printf("usage: B [+line] file...\n")
}

func main() {

	var (
		line    string
		files   []string
		sam     string
		display = os.Getenv("DISPLAY")
		user    = os.Getenv("USER")
		err     error
	)

	if len(os.Args) <= 1 {
		usage()
		os.Exit(1)
	}

	if validline.MatchString(os.Args[1]) {
		if len(os.Args) < 3 {
			usage()
			os.Exit(1)
		}
		line = strings.Replace(os.Args[1], "+", ":", 1)
		files = os.Args[2:]
	} else {
		files = os.Args[1:]
	}

	if len(files) == 0 {
		usage()
		os.Exit(1)
	}

	for i := 0; i < len(files); i++ {
		files[i], err = filepath.Abs(files[i])
		if err != nil {
			panic(err)
		}
	}

	if display == "" {
		sam = fmt.Sprintf("/tmp/.sam.%s", user)
	} else {
		if display == ":0" {
			display = ":0.0"
		}
		sam = fmt.Sprintf("/tmp/.sam.%s.%s", user, display)
	}

	if sam != "" {
		for _, f := range files {
			nf := fmt.Sprintf("%s%s", f, line)
			err = exec.Command("plumb", "-s", "B", "-d", "edit", nf).Run()
			if err != nil {
				panic(err)
			}
		}
	} else {
		fmt.Println("opening file in sam")
		sf, err := os.OpenFile(sam, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		defer sf.Close()
		for _, f := range files {
			nf := fmt.Sprintf("%s%s", f, line)
			_, err := sf.WriteString(fmt.Sprintf("B %s\n", nf))
			if err != nil {
				panic(err)
			}
			if line != "" && line != f {
				_, err := sf.WriteString(fmt.Sprintf("%s\n", line))
				if err != nil {
					panic(err)
				}
			}
		}
	}
}
