package cli

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/rest-sh/restish/v2/internal/request"
	"github.com/spf13/cobra"
)

func (c *CLI) addCertCommand(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:     "cert <uri>",
		Short:   "Show the TLS certificate chain for a server",
		Long:    certLong,
		GroupID: rootGroupUtility,
		Example: fmt.Sprintf(`  %s cert https://api.example.com
  %s cert api.example.com --warn-days 30`, c.commandNameOrDefault(), c.commandNameOrDefault()),
		Args: cobra.ExactArgs(1),
		RunE: c.runCert,
	}
	cmd.Flags().Int("warn-days", 0, "Exit non-zero if the leaf certificate expires within N days")
	root.AddCommand(cmd)
}

func (c *CLI) runCert(cmd *cobra.Command, args []string) error {
	if err := rejectResponseTransformFlags(cmd); err != nil {
		return err
	}
	opts, err := c.httpOptsFromFlags(cmd)
	if err != nil {
		return err
	}
	opts, err = c.resolveTLSSigner(opts)
	if err != nil {
		return err
	}

	targetURL, err := normalizeCertTarget(args[0], opts.Server)
	if err != nil {
		return err
	}
	u, err := url.Parse(targetURL)
	if err != nil {
		return err
	}
	if u.Scheme != "https" {
		return fmt.Errorf("cert: unsupported non-TLS scheme %q; use an https:// target", u.Scheme)
	}

	cfg, cleanup, err := request.TLSConfigWithCleanupFromOptions(opts)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup.Close()
	}
	if cfg.ServerName == "" {
		cfg.ServerName = u.Hostname()
	}

	hostPort := certDialAddress(u)

	dialer := &net.Dialer{Timeout: opts.Timeout}
	if dialer.Timeout == 0 {
		dialer.Timeout = 10 * time.Second
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", hostPort, cfg)
	if err != nil {
		return fmt.Errorf("cert: connect: %w", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(cmd.Context(), dialer.Timeout)
	defer cancel()
	if err := conn.HandshakeContext(ctx); err != nil {
		return fmt.Errorf("cert: handshake: %w", err)
	}

	state := conn.ConnectionState()
	var rendered strings.Builder
	for i, cert := range state.PeerCertificates {
		writeCertInfo(&rendered, i, cert)
	}
	data := []byte(rendered.String())
	if output.ColorEnabled(c.Stdout) {
		if lexer := lexers.Get("yaml"); lexer != nil {
			if colored, err := output.HighlightWithLexer(lexer, data); err == nil {
				data = colored
			}
		}
	}
	if _, err := c.Stdout.Write(data); err != nil {
		return err
	}

	warnDays, _ := cmd.Flags().GetInt("warn-days")
	if warnDays > 0 && len(state.PeerCertificates) > 0 {
		leaf := state.PeerCertificates[0]
		deadline := time.Now().Add(time.Duration(warnDays) * 24 * time.Hour)
		if leaf.NotAfter.Before(deadline) {
			fmt.Fprintf(c.Stderr, "warning: certificate for %s expires within %d days: %s expires at %s (%s)\n",
				u.Hostname(), warnDays, leaf.Subject.String(), leaf.NotAfter.Format(time.RFC3339), relativeExpiry(leaf.NotAfter))
			return &ExitCodeError{Code: 1}
		}
	}
	return nil
}

func normalizeCertTarget(rawURL, serverOverride string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if !strings.Contains(rawURL, "://") {
		if strings.HasPrefix(rawURL, ":") {
			rawURL = "https://localhost" + rawURL
		} else {
			rawURL = "https://" + rawURL
		}
	}
	return request.Normalize(rawURL, serverOverride)
}

func certDialAddress(u *url.URL) string {
	if _, _, err := net.SplitHostPort(u.Host); err == nil {
		return u.Host
	}
	return net.JoinHostPort(u.Hostname(), "443")
}

func writeCertInfo(w io.Writer, index int, cert *x509.Certificate) {
	label := "Leaf"
	if index > 0 {
		label = fmt.Sprintf("Chain %d", index)
	}
	fmt.Fprintf(w, "%s Certificate:\n", label)
	fmt.Fprintf(w, "  Subject: %s\n", cert.Subject.String())
	fmt.Fprintf(w, "  Issuer: %s\n", cert.Issuer.String())
	fmt.Fprintf(w, "  Valid From: %s\n", cert.NotBefore.Format(time.RFC3339))
	fmt.Fprintf(w, "  Valid Until: %s (%s)\n", cert.NotAfter.Format(time.RFC3339), relativeExpiry(cert.NotAfter))
	if len(cert.DNSNames) > 0 {
		fmt.Fprintf(w, "  DNS Names: %s\n", strings.Join(cert.DNSNames, ", "))
	}
	if len(cert.EmailAddresses) > 0 {
		fmt.Fprintf(w, "  Emails: %s\n", strings.Join(cert.EmailAddresses, ", "))
	}
	fmt.Fprintln(w)
}

func relativeExpiry(t time.Time) string {
	d := time.Until(t).Round(time.Hour)
	if d < 0 {
		d = -d
		return fmt.Sprintf("expired %s ago", d)
	}
	return fmt.Sprintf("expires in %s", d)
}
