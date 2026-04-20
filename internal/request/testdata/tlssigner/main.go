package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/danielgtaylor/restish/v2/plugin"
	"github.com/fxamacker/cbor/v2"
)

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--rsh-plugin-manifest" {
			data, err := cbor.Marshal(map[string]any{
				"name":                "test-tls-signer",
				"version":             "1.0.0",
				"description":         "Test TLS signer plugin",
				"restish_api_version": 2,
				"hooks":               []string{"tls-signer"},
			})
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			_, _ = os.Stdout.Write(data)
			return
		}
	}

	var init map[string]any
	dec := plugin.NewDecoder(os.Stdin)
	if err := dec.ReadMessage(&init); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if init["type"] != "init" {
		fmt.Fprintln(os.Stderr, "expected init message")
		os.Exit(1)
	}
	params, _ := init["params"].(map[string]any)

	certPath := os.Getenv("RSH_TLS_SIGNER_CERT")
	keyPath := os.Getenv("RSH_TLS_SIGNER_KEY")
	if text, _ := params["cert_path"].(string); text != "" {
		certPath = text
	}
	if text, _ := params["key_path"].(string); text != "" {
		keyPath = text
	}

	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		fmt.Fprintln(os.Stderr, "invalid certificate")
		os.Exit(1)
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		fmt.Fprintln(os.Stderr, "invalid private key")
		os.Exit(1)
	}
	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := plugin.WriteMessage(os.Stdout, map[string]any{
		"type":        "ready",
		"certificate": block.Bytes,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	mode := os.Getenv("RSH_TLS_SIGNER_MODE")
	for {
		var msg map[string]any
		if err := dec.ReadMessage(&msg); err != nil {
			os.Exit(1)
		}
		if msg["type"] == plugin.MsgTypeTLSSignerShutdown {
			if shutdownFile := os.Getenv("RSH_TLS_SIGNER_SHUTDOWN_FILE"); shutdownFile != "" {
				_ = os.WriteFile(shutdownFile, []byte("shutdown"), 0o644)
			}
			return
		}
		if msg["type"] != "sign" {
			continue
		}
		if mode == "die" {
			os.Exit(1)
		}
		if mode == "error" {
			_ = plugin.WriteMessage(os.Stdout, map[string]any{"error": "device removed"})
			continue
		}
		if mode == "stderr-error" {
			fmt.Fprintln(os.Stderr, "pin incorrect")
			_ = plugin.WriteMessage(os.Stdout, map[string]any{"error": "sign failed"})
			continue
		}
		digest := msgBytes(msg["digest"])
		hash := crypto.Hash(msgInt(msg["hash"]))
		var sig []byte
		var err error
		if padding, _ := msg["padding"].(string); padding == "pss" {
			sig, err = rsa.SignPSS(rand.Reader, key, hash, digest, &rsa.PSSOptions{
				SaltLength: msgInt(msg["salt_length"]),
				Hash:       hash,
			})
		} else {
			sig, err = rsa.SignPKCS1v15(rand.Reader, key, hash, digest)
		}
		if err != nil {
			_ = plugin.WriteMessage(os.Stdout, map[string]any{"error": err.Error()})
			continue
		}
		_ = plugin.WriteMessage(os.Stdout, map[string]any{"signature": sig})
	}
}

func msgBytes(v any) []byte {
	switch data := v.(type) {
	case []byte:
		return data
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

func msgInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case uint64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}
