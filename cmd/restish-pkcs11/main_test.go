package main

import (
	"crypto/tls"
	"testing"
)

func TestCertificateDER(t *testing.T) {
	got, err := certificateDER(&tls.Certificate{
		Certificate: [][]byte{{0x01, 0x02, 0x03}},
	})
	if err != nil {
		t.Fatalf("certificateDER: %v", err)
	}
	if len(got) != 3 || got[0] != 0x01 || got[2] != 0x03 {
		t.Fatalf("unexpected der: %v", got)
	}
}

func TestCertificateDERRequiresLeaf(t *testing.T) {
	if _, err := certificateDER(&tls.Certificate{}); err == nil {
		t.Fatal("expected error for empty certificate")
	}
}
