package main

import (
	"errors"
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "loadtest:", err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	if len(arguments) == 0 {
		return usageError()
	}
	switch arguments[0] {
	case "prepare":
		return runPrepare(arguments[1:])
	case "cleanup":
		return runCleanup(arguments[1:])
	case "http":
		return runHTTP(arguments[1:])
	case "private":
		return runPrivate(arguments[1:])
	case "group":
		return runGroup(arguments[1:])
	case "file":
		return runFile(arguments[1:])
	default:
		return usageError()
	}
}

func usageError() error {
	return errors.New("usage: loadtest <prepare|cleanup|http|private|group|file> [flags]")
}
