package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

// addLinksCommand registers the "links" subcommand on root.
func (c *CLI) addLinksCommand(root *cobra.Command) {
	root.AddCommand(&cobra.Command{
		Use:     "links <uri> [rel...]",
		Short:   "GET a URI and display its hypermedia links",
		GroupID: rootGroupUtility,
		Example: fmt.Sprintf(`  %s links https://api.example.com/items/123
  %s links https://api.example.com/items/123 self next`, c.commandNameOrDefault(), c.commandNameOrDefault()),
		Long: linksLong,
		Args: cobra.MinimumNArgs(1),
		RunE: c.runLinksCmd,
	})
}

// runLinksCmd performs a GET and prints the parsed hypermedia links.
func (c *CLI) runLinksCmd(cmd *cobra.Command, args []string) error {
	uri := args[0]
	filterRels := args[1:]

	if _, err := commandJSONOutputRequested(cmd); err != nil {
		return err
	}

	opts, err := c.httpOptsFromFlags(cmd)
	if err != nil {
		return err
	}

	profileName := c.profileFromCmd(cmd)
	prepared, err := c.prepareRequest(requestContext(cmd), "GET", uri, profileName, opts, nil, nil, false, authHandlerOptions{}, nil, false, "")
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

	links := map[string]string{}
	if len(resp.Links) > 0 {
		links = make(map[string]string, len(resp.Links))
		for rel, value := range resp.Links {
			if href, ok := value.(string); ok {
				links[rel] = href
			}
		}
	}

	// Filter to requested rels if specified.
	if len(filterRels) > 0 {
		filtered := make(map[string]string, len(filterRels))
		for _, rel := range filterRels {
			if u, ok := links[rel]; ok {
				filtered[rel] = u
			} else {
				c.warnf("rel %q not found (available: %v)", rel, sortedLinkRels(links))
			}
		}
		links = filtered
	}

	if err := c.writePrettyJSON(links); err != nil {
		return err
	}
	return c.statusError(cmd, resp.Status)
}

func sortedLinkRels(links map[string]string) []string {
	rels := make([]string, 0, len(links))
	for rel := range links {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	return rels
}
