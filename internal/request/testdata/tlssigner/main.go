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
				"restish_api_version": 1,
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

	certPEM, err := os.ReadFile(os.Getenv("RSH_TLS_SIGNER_CERT"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	keyPEM, err := os.ReadFile(os.Getenv("RSH_TLS_SIGNER_KEY"))
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
		if err := plugin.ReadMessage(os.Stdin, &msg); err != nil {
			os.Exit(1)
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
