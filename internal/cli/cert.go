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
		Use:   "cert <uri>",
		Short: "Show the TLS certificate chain for a server",
		Args:  cobra.ExactArgs(1),
		RunE:  c.runCert,
	}
	cmd.Flags().Int("warn-days", 0, "Exit non-zero if the leaf certificate expires within N days")
	root.AddCommand(cmd)
}

func (c *CLI) runCert(cmd *cobra.Command, args []string) error {
	opts, err := c.httpOptsFromFlags(cmd)
	if err != nil {
		return err
	}
	opts, err = c.resolveTLSSigner(opts)
	if err != nil {
		return err
	}

	targetURL, err := request.Normalize(args[0], opts.Server)
	if err != nil {
		return err
	}
	u, err := url.Parse(targetURL)
	if err != nil {
		return err
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

	hostPort := u.Host
	if !strings.Contains(hostPort, ":") {
		hostPort = net.JoinHostPort(u.Hostname(), "443")
	}

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
		deadline := time.Now().Add(time.Duration(warnDays) * 24 * time.Hour)
		if state.PeerCertificates[0].NotAfter.Before(deadline) {
			return &ExitCodeError{Code: 1}
		}
	}
	return nil
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
