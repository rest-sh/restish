package main

import (
	"fmt"
	"os"

	restish "github.com/rest-sh/restish/v2"
)

func main() {
	cli := restish.New()
	cli.SetCommandName("example")
	cli.SetCommandDescription("Example CLI", "Example CLI for api.rest.sh.")
	cli.SetDefaultConfig(&restish.Config{APIs: map[string]*restish.APIConfig{
		"api": {
			BaseURL: "https://api.rest.sh",
			SpecURL: "https://api.rest.sh/openapi.json",
		},
	}})
	cli.SetCommandSurface(restish.CommandSurface{
		PromotedAPI: "api",
	})

	if err := cli.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
