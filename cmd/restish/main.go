package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/danielgtaylor/restish/v2/internal/cli"
)

func main() {
	c := cli.New()
	if err := c.Run(os.Args); err != nil {
		// ExitCodeError means the HTTP response was already printed; just
		// exit with the mapped status code without printing anything extra.
		var exitErr *cli.ExitCodeError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
