package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/danielgtaylor/restish/v2/internal/hypermedia"
	"github.com/danielgtaylor/restish/v2/internal/output"
	"github.com/danielgtaylor/restish/v2/internal/request"
	"github.com/spf13/cobra"
)

// addLinksCommand registers the "links" subcommand on root.
func (c *CLI) addLinksCommand(root *cobra.Command) {
	root.AddCommand(&cobra.Command{
		Use:   "links <uri> [rel...]",
		Short: "GET a URI and display its hypermedia links",
		Long: `Performs a GET request to <uri> and prints all hypermedia links
found in the response (Link headers, HAL _links, JSON:API links, Siren links,
JSON-LD @id). Optionally filter to specific relation types.`,
		Args: cobra.MinimumNArgs(1),
		RunE: c.runLinksCmd,
	})
}

// runLinksCmd performs a GET and prints the parsed hypermedia links.
func (c *CLI) runLinksCmd(cmd *cobra.Command, args []string) error {
	uri := args[0]
	filterRels := args[1:]

	opts, err := c.httpOptsFromFlags(cmd)
	if err != nil {
		return err
	}

	profileName, _ := cmd.Flags().GetString("rsh-profile")
	if profileName == "" {
		profileName = os.Getenv("RSH_PROFILE")
	}
	if profileName == "" {
		profileName = "default"
	}
	uri, _, opts = c.applyAPIProfile(uri, profileName, opts)

	httpResp, err := request.Do(context.Background(), "GET", uri, nil, opts)
	if err != nil {
		return fmt.Errorf("network: %w", err)
	}

	resp, err := output.Normalize(httpResp, c.content)
	if err != nil {
		return err
	}

	var links map[string]string
	if httpResp.Request != nil {
		links = hypermedia.Parse(httpResp.Request.URL, httpResp.Header, resp.Body, c.linkParsers)
	}

	// Filter to requested rels if specified.
	if len(filterRels) > 0 && len(links) > 0 {
		filtered := make(map[string]string, len(filterRels))
		for _, rel := range filterRels {
			if u, ok := links[rel]; ok {
				filtered[rel] = u
			}
		}
		links = filtered
	}

	data, err := json.MarshalIndent(links, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(c.Stdout, string(data))
	return nil
}
