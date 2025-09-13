package main

import (
	"io"
	"os"
	"strconv"
)

func g[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func main() {
	exit := 0
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "stdout":
			g(os.Stdout.WriteString(os.Args[i+1]))
		case "stderr":
			g(os.Stderr.WriteString(os.Args[i+1]))
		case "stdin":
			value := ""
			buf := []byte{0}
			for read, err := os.Stdin.Read(buf); read >= 0 && err != io.EOF; read, err = os.Stdin.Read(buf) {
				value += string(buf[0])
			}
			g(os.Stderr.WriteString(value))
		case "exit-code":
			exit = g(strconv.Atoi(os.Args[i+1]))
		}
		i++
	}
	os.Exit(exit)
}
