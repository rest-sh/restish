package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"strings"
	"testing"
)

func TestCappedBufferLimitsStoredBytes(t *testing.T) {
	buf := &cappedBuffer{limit: 4}
	n, err := buf.Write([]byte("abcdef"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != 6 {
		t.Fatalf("Write count = %d, want 6", n)
	}
	if got := buf.Bytes(); !bytes.Equal(got, []byte("abcd")) {
		t.Fatalf("stored bytes = %q, want %q", got, []byte("abcd"))
	}
	if !buf.Truncated() {
		t.Fatal("expected buffer to report truncation")
	}
}

func TestMaterializePluginDownloadRejectsOversizedDirectBinary(t *testing.T) {
	restorePluginInstallLimits(t, pluginInstallSizeLimits{
		DownloadBytes:       4,
		ArchiveMemberBytes:  4,
		ArchiveExtractBytes: 8,
	})

	_, err := materializePluginDownload(strings.NewReader("abcdef"), "https://downloads.example/restish-test", t.TempDir(), "restish-test")
	if err == nil {
		t.Fatal("expected oversized direct binary to fail")
	}
	if !strings.Contains(err.Error(), "exceeds limit") {
		t.Fatalf("expected size limit error, got %v", err)
	}
}

func TestExtractPluginTarGzRejectsOversizedEntry(t *testing.T) {
	restorePluginInstallLimits(t, pluginInstallSizeLimits{
		DownloadBytes:       1024,
		ArchiveMemberBytes:  4,
		ArchiveExtractBytes: 8,
	})

	archive := testTarGzEntry(t, "restish-test", []byte("abcde"))
	_, err := materializePluginDownload(bytes.NewReader(archive), "https://downloads.example/restish-test.tar.gz", t.TempDir(), "restish-test")
	if err == nil {
		t.Fatal("expected oversized tar.gz entry to fail")
	}
	if !strings.Contains(err.Error(), "archive member exceeds limit") {
		t.Fatalf("expected archive member limit error, got %v", err)
	}
}

func TestExtractPluginZipRejectsOversizedEntry(t *testing.T) {
	restorePluginInstallLimits(t, pluginInstallSizeLimits{
		DownloadBytes:       1024,
		ArchiveMemberBytes:  4,
		ArchiveExtractBytes: 8,
	})

	archive := testZipEntry(t, "restish-test", []byte("abcde"))
	_, err := materializePluginDownload(bytes.NewReader(archive), "https://downloads.example/restish-test.zip", t.TempDir(), "restish-test")
	if err == nil {
		t.Fatal("expected oversized zip entry to fail")
	}
	if !strings.Contains(err.Error(), "archive member exceeds limit") {
		t.Fatalf("expected archive member limit error, got %v", err)
	}
}

func TestExtractPluginArchiveRejectsTotalExtractedSize(t *testing.T) {
	restorePluginInstallLimits(t, pluginInstallSizeLimits{
		DownloadBytes:       1024,
		ArchiveMemberBytes:  4,
		ArchiveExtractBytes: 5,
	})

	archive := testZipEntries(t, map[string][]byte{
		"restish-one": []byte("abc"),
		"restish-two": []byte("def"),
	})
	_, err := materializePluginDownload(bytes.NewReader(archive), "https://downloads.example/plugins.zip", t.TempDir(), "")
	if err == nil {
		t.Fatal("expected total extracted size to fail")
	}
	if !strings.Contains(err.Error(), "archive exceeds extracted limit") {
		t.Fatalf("expected archive total limit error, got %v", err)
	}
}

func restorePluginInstallLimits(t *testing.T, limits pluginInstallSizeLimits) {
	t.Helper()
	old := pluginInstallLimits
	pluginInstallLimits = limits
	t.Cleanup(func() { pluginInstallLimits = old })
}

func testTarGzEntry(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(data))}); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatalf("write tar data: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

func testZipEntry(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	return testZipEntries(t, map[string][]byte{name: data})
}

func testZipEntries(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := io.Copy(w, bytes.NewReader(data)); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}
