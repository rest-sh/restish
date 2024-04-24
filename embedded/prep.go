package embedded

import (
	"fmt"
	"io"
	"os"

	"github.com/danielgtaylor/restish/bulk"
	"github.com/danielgtaylor/restish/cli"
	"github.com/danielgtaylor/restish/oauth"
	"github.com/danielgtaylor/restish/openapi"
	"github.com/spf13/viper"
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
	os.Args = []string{"restish-embedded"}
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
		viper.Set("ni-override-auth-prefix", overrideAuthPrefix)
		viper.Set("ni-override-auth-token", overrideAuthToken)
	}
	if newOut != nil {
		cli.Root.SetOut(newOut)
		cli.Stdout = newOut
	}
	if newErr != nil {
		cli.Root.SetErr(newErr)
		cli.Stderr = newErr
	}
	// Run the CLI, parsing arguments, making requests, and printing responses.
	if err := cli.Run(); err != nil {
		return fmt.Errorf("%w %v", err, cli.GetExitCode())
	}
	return nil
}
