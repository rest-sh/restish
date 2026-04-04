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
func TLSCertificateFromPlugin(path string) (*tls.Certificate, error) {
	stdout, stdin, stderr, proc, err := startTLSSigner(path)
	if err != nil {
		return nil, err
	}

	var ready map[string]any
	if err := readTLSSignerMessage(stdout, &ready); err != nil {
		_ = proc.Process.Kill()
		return nil, fmt.Errorf("tls-signer %s: ready: %w", filepath.Base(path), err)
	}
	if msgType, _ := ready["type"].(string); msgType != "ready" {
		_ = proc.Process.Kill()
		return nil, fmt.Errorf("tls-signer %s: expected ready message, got %q", filepath.Base(path), msgType)
	}

	der := msgBytes(ready["certificate"])
	if len(der) == 0 {
		_ = proc.Process.Kill()
		return nil, fmt.Errorf("tls-signer %s: ready message missing certificate", filepath.Base(path))
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		_ = proc.Process.Kill()
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

type PluginSigner struct {
	path   string
	stdin  io.WriteCloser
	stdout io.Reader
	stderr *bytes.Buffer
	proc   *exec.Cmd
	pub    crypto.PublicKey

	mu sync.Mutex
}

func (s *PluginSigner) Public() crypto.PublicKey {
	return s.pub
}

func (s *PluginSigner) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.proc.ProcessState != nil && s.proc.ProcessState.Exited() {
		return nil, fmt.Errorf("tls-signer %s: process exited", filepath.Base(s.path))
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
		return nil, fmt.Errorf("tls-signer %s: write sign request: %w", filepath.Base(s.path), err)
	}

	var reply map[string]any
	if err := readTLSSignerMessage(s.stdout, &reply); err != nil {
		return nil, fmt.Errorf("tls-signer %s: sign reply: %w", filepath.Base(s.path), err)
	}
	if text, _ := reply["error"].(string); text != "" {
		return nil, fmt.Errorf("tls-signer %s: %s", filepath.Base(s.path), text)
	}
	sig := msgBytes(reply["signature"])
	if len(sig) == 0 {
		return nil, fmt.Errorf("tls-signer %s: sign reply missing signature", filepath.Base(s.path))
	}
	return sig, nil
}

func startTLSSigner(path string) (io.Reader, io.WriteCloser, *bytes.Buffer, *exec.Cmd, error) {
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

func readTLSSignerMessage(r io.Reader, out any) error {
	type result struct{ err error }
	done := make(chan result, 1)
	go func() {
		done <- result{err: pluginwire.ReadMessage(r, out)}
	}()
	select {
	case res := <-done:
		return res.err
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timed out waiting for plugin reply")
	}
}

func msgBytes(v any) []byte {
	switch data := v.(type) {
	case []byte:
		return data
	case string:
		return []byte(data)
	case []any:
		out := make([]byte, 0, len(data))
		for _, item := range data {
			switch n := item.(type) {
			case uint64:
				out = append(out, byte(n))
			case int64:
				out = append(out, byte(n))
			case int:
				out = append(out, byte(n))
			}
		}
		return out
	default:
		return nil
	}
}
