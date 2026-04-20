package plugin

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	pluginwire "github.com/danielgtaylor/restish/v2/plugin"
)

// TLSCertificateFromPlugin starts a tls-signer plugin, waits for its ready
// message, and returns a tls.Certificate whose PrivateKey proxies Sign calls
// back to the plugin.
func TLSCertificateFromPlugin(path string, params map[string]string) (*tls.Certificate, error) {
	stdout, stdin, stderr, proc, err := startTLSSigner(path)
	if err != nil {
		return nil, err
	}

	// cleanup kills the process and releases all associated resources. It is
	// called on every error path before TLSCertificateFromPlugin returns.
	// Closing stdin before Kill allows the plugin to observe EOF and shut down
	// gracefully on platforms where Kill delivery is not instantaneous.
	cleanup := func() {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = proc.Process.Kill()
		_ = proc.Wait()
	}

	if err := pluginwire.WriteMessage(stdin, pluginwire.TLSSignerInitMsg{
		Type:   pluginwire.MsgTypeInit,
		Params: params,
	}); err != nil {
		cleanup()
		return nil, fmt.Errorf("tls-signer %s: init: %w", filepath.Base(path), err)
	}

	dec := pluginwire.NewDecoder(stdout)
	var ready pluginwire.TLSSignerReadyMsg
	if err := readTLSSignerMessage(dec, stdout, &ready); err != nil {
		cleanup()
		return nil, fmt.Errorf("tls-signer %s: ready: %w", filepath.Base(path), err)
	}
	if ready.Type != pluginwire.MsgTypeTLSSignerReady {
		cleanup()
		return nil, fmt.Errorf("tls-signer %s: expected ready message, got %q", filepath.Base(path), ready.Type)
	}

	der := ready.Certificate
	if len(der) == 0 {
		cleanup()
		return nil, fmt.Errorf("tls-signer %s: ready message missing certificate", filepath.Base(path))
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("tls-signer %s: parse certificate: %w", filepath.Base(path), err)
	}

	signer := &PluginSigner{
		path:   path,
		stdin:  stdin,
		stdout: stdout,
		dec:    dec,
		stderr: stderr,
		proc:   proc,
		pub:    cert.PublicKey,
	}
	tlsCert := &tls.Certificate{
		Certificate: [][]byte{der},
		Leaf:        cert,
		PrivateKey:  signer,
	}
	return tlsCert, nil
}

// PluginSigner implements crypto.Signer by delegating Sign operations to a
// long-lived tls-signer plugin subprocess.
type PluginSigner struct {
	path   string
	stdin  io.WriteCloser
	stdout io.ReadCloser
	dec    *pluginwire.Decoder
	stderr *bytes.Buffer
	proc   *exec.Cmd
	pub    crypto.PublicKey

	mu   sync.Mutex
	dead bool // set to true after any fatal error; guarded by mu
}

// shutdown kills the plugin process and releases its resources.
// It must be called with mu held. Subsequent calls are no-ops.
func (s *PluginSigner) shutdown() {
	_ = s.closeLocked(false)
}

// Close gracefully shuts down the signer subprocess.
func (s *PluginSigner) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closeLocked(true)
}

func (s *PluginSigner) closeLocked(graceful bool) error {
	if s.dead {
		return nil
	}
	s.dead = true
	if graceful {
		_ = pluginwire.WriteMessage(s.stdin, pluginwire.TLSSignerShutdownMsg{Type: pluginwire.MsgTypeTLSSignerShutdown})
	}
	_ = s.stdin.Close()

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- s.proc.Wait()
	}()

	var waitErr error
	select {
	case waitErr = <-waitCh:
	case <-time.After(5 * time.Second):
		_ = s.proc.Process.Kill()
		waitErr = <-waitCh
	}

	_ = s.stdout.Close()
	return waitErr
}

func (s *PluginSigner) Public() crypto.PublicKey {
	return s.pub
}

func (s *PluginSigner) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.dead {
		return nil, fmt.Errorf("tls-signer %s: process has exited", filepath.Base(s.path))
	}

	hash := crypto.Hash(0)
	padding := ""
	saltLength := 0
	if opts != nil {
		hash = opts.HashFunc()
		if pss, ok := opts.(*rsa.PSSOptions); ok {
			padding = "pss"
			saltLength = pss.SaltLength
		}
	}
	if err := pluginwire.WriteMessage(s.stdin, pluginwire.TLSSignerSignMsg{
		Type:       pluginwire.MsgTypeTLSSignerSign,
		Digest:     append([]byte(nil), digest...),
		Hash:       uint64(hash),
		Padding:    padding,
		SaltLength: saltLength,
	}); err != nil {
		s.shutdown()
		return nil, s.signError(fmt.Sprintf("write sign request: %v", err))
	}

	var reply pluginwire.TLSSignerSignedMsg
	if err := readTLSSignerMessage(s.dec, s.stdout, &reply); err != nil {
		s.shutdown()
		return nil, s.signError(fmt.Sprintf("sign reply: %v", err))
	}
	if reply.Error != "" {
		return nil, s.signError(reply.Error)
	}
	if len(reply.Signature) == 0 {
		return nil, fmt.Errorf("tls-signer %s: sign reply missing signature", filepath.Base(s.path))
	}
	return reply.Signature, nil
}

func (s *PluginSigner) signError(detail string) error {
	msg := fmt.Sprintf("tls-signer %s: %s", filepath.Base(s.path), detail)
	if s.stderr != nil {
		if stderr := strings.TrimSpace(s.stderr.String()); stderr != "" {
			msg += "\nstderr: " + stderr
		}
	}
	return fmt.Errorf("%s", msg)
}

func startTLSSigner(path string) (io.ReadCloser, io.WriteCloser, *bytes.Buffer, *exec.Cmd, error) {
	proc := exec.Command(path)
	stdin, err := proc.StdinPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("tls-signer %s: stdin pipe: %w", filepath.Base(path), err)
	}
	stdout, err := proc.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, nil, nil, nil, fmt.Errorf("tls-signer %s: stdout pipe: %w", filepath.Base(path), err)
	}
	var stderr bytes.Buffer
	proc.Stderr = &stderr
	if err := proc.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, nil, nil, nil, fmt.Errorf("tls-signer %s: start: %w", filepath.Base(path), err)
	}
	return stdout, stdin, &stderr, proc, nil
}

// readTLSSignerMessage reads one CBOR data item from dec with a 10-second timeout.
//
// If the deadline fires, closer is closed to unblock the background reader
// goroutine and the function waits for that goroutine to exit before returning
// — so there is no goroutine leak regardless of whether the read succeeded or
// timed out. Callers should treat any error as fatal and shut down the signer
// process.
func readTLSSignerMessage(dec *pluginwire.Decoder, closer io.Closer, out any) error {
	type result struct{ err error }
	done := make(chan result, 1)
	go func() {
		done <- result{err: dec.ReadMessage(out)}
	}()
	select {
	case res := <-done:
		return res.err
	case <-time.After(10 * time.Second):
		_ = closer.Close() // unblocks the goroutine above
		<-done             // wait for it to exit — no leak
		return fmt.Errorf("timed out waiting for plugin reply")
	}
}
