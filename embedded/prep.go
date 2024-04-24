package embedded

import (
	"fmt"
	"io"
	"os"

	"github.com/loicalleyne/restish/bulk"
	"github.com/loicalleyne/restish/cli"
	"github.com/loicalleyne/restish/oauth"
	"github.com/loicalleyne/restish/openapi"
)

var version string = "embedded"

func Restish(appName string, args []string, overrideAuthPrefix, overrideAuthToken string, newOut, newErr io.Writer) error {
	switch appName {
	case "":
		return fmt.Errorf("no app name provided")
	default:
		cli.Init(appName, version)
	}
	osArgsBackup := os.Args
	defer func() { os.Args = osArgsBackup }()
	os.Args = []string{"embedded"}
	os.Args = append(os.Args, args...)
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

	// Run the CLI, parsing arguments, making requests, and printing responses.
	if err := cli.RunEmbedded(args, overrideAuthPrefix, overrideAuthToken, newOut, newErr); err != nil {
		return fmt.Errorf("%w %v", err, cli.GetExitCode())
	}
	return nil
}
