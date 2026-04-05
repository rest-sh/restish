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

	if err := pluginwire.WriteMessage(stdin, map[string]any{
		"type":   "init",
		"params": params,
	}); err != nil {
		cleanup()
		return nil, fmt.Errorf("tls-signer %s: init: %w", filepath.Base(path), err)
	}

	var ready map[string]any
	if err := readTLSSignerMessage(stdout, &ready); err != nil {
		cleanup()
		return nil, fmt.Errorf("tls-signer %s: ready: %w", filepath.Base(path), err)
	}
	if msgType, _ := ready["type"].(string); msgType != "ready" {
		cleanup()
		return nil, fmt.Errorf("tls-signer %s: expected ready message, got %q", filepath.Base(path), msgType)
	}

	der := MsgBytes(ready["certificate"])
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
	stderr *bytes.Buffer
	proc   *exec.Cmd
	pub    crypto.PublicKey

	mu   sync.Mutex
	dead bool // set to true after any fatal error; guarded by mu
}

// shutdown kills the plugin process and releases its resources.
// It must be called with mu held. Subsequent calls are no-ops.
func (s *PluginSigner) shutdown() {
	if s.dead {
		return
	}
	s.dead = true
	_ = s.stdin.Close()
	_ = s.stdout.Close()
	_ = s.proc.Process.Kill()
	_ = s.proc.Wait()
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
	msg := map[string]any{
		"type":   "sign",
		"digest": append([]byte(nil), digest...),
		"hash":   uint64(hash),
	}
	if padding != "" {
		msg["padding"] = padding
		msg["salt_length"] = saltLength
	}
	if err := pluginwire.WriteMessage(s.stdin, msg); err != nil {
		s.shutdown()
		return nil, fmt.Errorf("tls-signer %s: write sign request: %w", filepath.Base(s.path), err)
	}

	var reply map[string]any
	if err := readTLSSignerMessage(s.stdout, &reply); err != nil {
		s.shutdown()
		return nil, fmt.Errorf("tls-signer %s: sign reply: %w", filepath.Base(s.path), err)
	}
	if text, _ := reply["error"].(string); text != "" {
		return nil, fmt.Errorf("tls-signer %s: %s", filepath.Base(s.path), text)
	}
	sig := MsgBytes(reply["signature"])
	if len(sig) == 0 {
		return nil, fmt.Errorf("tls-signer %s: sign reply missing signature", filepath.Base(s.path))
	}
	return sig, nil
}

func startTLSSigner(path string) (io.ReadCloser, io.WriteCloser, *bytes.Buffer, *exec.Cmd, error) {
	proc := exec.Command(path)
	stdin, err := proc.StdinPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("tls-signer %s: stdin pipe: %w", filepath.Base(path), err)
	}
	stdout, err := proc.StdoutPipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("tls-signer %s: stdout pipe: %w", filepath.Base(path), err)
	}
	var stderr bytes.Buffer
	proc.Stderr = &stderr
	if err := proc.Start(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("tls-signer %s: start: %w", filepath.Base(path), err)
	}
	return stdout, stdin, &stderr, proc, nil
}

// readTLSSignerMessage reads one CBOR message from r with a 10-second timeout.
//
// If the deadline fires, r is closed to unblock the background reader goroutine
// and the function waits for that goroutine to exit before returning — so there
// is no goroutine leak regardless of whether the read succeeded or timed out.
// Callers should treat any error as fatal and shut down the signer process.
func readTLSSignerMessage(r io.ReadCloser, out any) error {
	type result struct{ err error }
	done := make(chan result, 1)
	go func() {
		done <- result{err: pluginwire.ReadMessage(r, out)}
	}()
	select {
	case res := <-done:
		return res.err
	case <-time.After(10 * time.Second):
		_ = r.Close() // unblocks the goroutine above
		<-done        // wait for it to exit — no leak
		return fmt.Errorf("timed out waiting for plugin reply")
	}
}
