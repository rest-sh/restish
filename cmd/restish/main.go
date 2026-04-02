package main

import (
	"fmt"
	"os"

	"github.com/danielgtaylor/restish/v2/internal/cli"
)

func main() {
	c := cli.New()
	if err := c.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
