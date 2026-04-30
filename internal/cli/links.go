package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// addLinksCommand registers the "links" subcommand on root.
func (c *CLI) addLinksCommand(root *cobra.Command) {
	root.AddCommand(&cobra.Command{
		Use:     "links <uri> [rel...]",
		Short:   "GET a URI and display its hypermedia links",
		GroupID: rootGroupUtility,
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

	profileName := c.profileFromCmd(cmd)
	prepared, err := c.prepareRequest(uri, profileName, opts, nil, nil, false, authHandlerOptions{}, nil)
	if err != nil {
		return err
	}
	defer c.closePreparedTransport(prepared)

	httpResp, err := c.sendPreparedRequest(requestContext(cmd), "GET", prepared)
	if err != nil {
		return fmt.Errorf("network: %w", err)
	}

	resp, err := c.normalizeHTTPResponse(httpResp, maxBodyBytes(cmd))
	if err != nil {
		return err
	}
	c.ensureBodyLinks(resp)

	var links map[string]string
	if len(resp.Links) > 0 {
		links = make(map[string]string, len(resp.Links))
		for rel, value := range resp.Links {
			if href, ok := value.(string); ok {
				links[rel] = href
			}
		}
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
	return c.statusError(cmd, resp.Status)
}
