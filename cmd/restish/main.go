package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/rest-sh/restish/v2/internal/cli"
)

func main() {
	c := cli.New()
	if err := c.Run(os.Args); err != nil {
		// ExitCodeError means the HTTP response was already printed (or the exit
		// code carries the full meaning, e.g. 130 for SIGINT); just exit with the
		// mapped status code without printing anything extra.
		var exitErr *cli.ExitCodeError
		if errors.As(err, &exitErr) {
			if exitErr.Cause != nil {
				fmt.Fprintln(os.Stderr, exitErr.Cause)
			}
			os.Exit(exitErr.Code)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
