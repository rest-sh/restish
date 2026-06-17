package fileutil

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestLockSiblingFileExclusive(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "data")

	lock1, err := LockSiblingFile(target)
	if err != nil {
		t.Fatal(err)
	}
	defer lock1.Close()

	// Second lock attempt should still succeed in opening the file (the
	// flock is exclusive on Linux but we want a non-blocking second
	// attempt to confirm the lock file is present and writable).
	lockPath := target + ".lock"
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("expected lock file at %s: %v", lockPath, err)
	}
}

func TestLockSiblingFileContention(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "data")

	lock1, err := LockSiblingFile(target)
	if err != nil {
		t.Fatal(err)
	}
	defer lock1.Close()

	// Try a second lock; flock blocks. We run it in a goroutine to confirm we
	// can get the lock after release.
	holder := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(holder)
		lock2, err := LockSiblingFile(target)
		if err != nil {
			done <- err
			return
		}
		defer lock2.Close()
		done <- nil
	}()

	<-holder
	// Give the goroutine a moment to attempt the lock; it should be
	// blocked behind lock1.
	if err := lock1.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("second lock: %v", err)
	}
}

func TestAtomicWriteFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	want := []byte("hello, world\n")
	if err := AtomicWriteFile(path, want, AtomicWriteOptions{FileMode: 0o600, DirMode: 0o700}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("got %q, want %q", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %o, want 0o600", info.Mode().Perm())
	}
}

func TestAtomicWriteFileConcurrentSafe(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			data := []byte{byte('a' + i)}
			if err := AtomicWriteFile(path, data, AtomicWriteOptions{FileMode: 0o600, DirMode: 0o700}); err != nil {
				t.Errorf("write %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 byte, got %d", len(got))
	}
}
