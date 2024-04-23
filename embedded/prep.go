package embedded

import (
	"fmt"
	"io"
	"strings"

	"github.com/danielgtaylor/restish/bulk"
	"github.com/danielgtaylor/restish/cli"
	"github.com/danielgtaylor/restish/oauth"
	"github.com/danielgtaylor/restish/openapi"
)

var version string = "embedded"

func Restish(args []string, overrideAuthPrefix, overrideAuthToken string, newOut, newErr io.Writer) error {

	cli.Init("restish", version)

	// Register default encodings, content type handlers, and link parsers.
	cli.Defaults()

	bulk.Init(cli.Root)

	// Register format loaders to auto-discover API descriptions
	cli.AddLoader(openapi.New())

	// Register auth schemes
	cli.AddAuth("oauth-client-credentials", &oauth.ClientCredentialsHandler{})
	cli.AddAuth("oauth-authorization-code", &oauth.AuthorizationCodeHandler{})
	if overrideAuthToken != "" {
		cli.AddAuth("override", &cli.ExternalOverrideAuth{})
	}
	// We need to register new commands at runtime based on the selected API
	// so that we don't have to potentially refresh and parse every single
	// registered API just to run. So this is a little hacky, but we hijack
	// the input args to find non-option arguments, get the first arg, and
	// if it isn't from a well-known set try to load that API.
	runArgs := []string{}
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "__") {
			runArgs = append(runArgs, arg)
		}
	}
	// Run the CLI, parsing arguments, making requests, and printing responses.
	if err := cli.RunEmbedded(runArgs, overrideAuthPrefix, overrideAuthToken, newOut, newErr); err != nil {
		return fmt.Errorf("%w %v", err, cli.GetExitCode())
	}
	return nil
}
