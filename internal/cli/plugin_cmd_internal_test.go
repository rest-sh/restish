package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strconv"
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

func TestExtractPluginTarGzRejectsMalformedOversizedEntryByReadLimit(t *testing.T) {
	restorePluginInstallLimits(t, pluginInstallSizeLimits{
		DownloadBytes:       1024,
		ArchiveMemberBytes:  4,
		ArchiveExtractBytes: 32,
	})

	archive := testMalformedTarGzEntry(t, "restish-test", 10, []byte("abcde"))
	_, err := materializePluginDownload(bytes.NewReader(archive), "https://downloads.example/restish-test.tar.gz", t.TempDir(), "restish-test")
	if err == nil {
		t.Fatal("expected malformed oversized tar.gz entry to fail")
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

func testMalformedTarGzEntry(t *testing.T, name string, declaredSize int64, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	header := make([]byte, 512)
	copy(header[0:100], name)
	writeTarOctal(header[100:108], 0o755)
	writeTarOctal(header[108:116], 0)
	writeTarOctal(header[116:124], 0)
	writeTarOctal(header[124:136], declaredSize)
	writeTarOctal(header[136:148], 0)
	for i := 148; i < 156; i++ {
		header[i] = ' '
	}
	header[156] = '0'
	copy(header[257:263], "ustar\x00")
	copy(header[263:265], "00")
	var sum int64
	for _, b := range header {
		sum += int64(b)
	}
	copy(header[148:156], fmt.Sprintf("%06o\x00 ", sum))
	if _, err := gz.Write(header); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := gz.Write(data); err != nil {
		t.Fatalf("write tar data: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

func writeTarOctal(field []byte, n int64) {
	for i := range field {
		field[i] = '0'
	}
	s := strconv.FormatInt(n, 8)
	copy(field[len(field)-1-len(s):], s)
	field[len(field)-1] = 0
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
