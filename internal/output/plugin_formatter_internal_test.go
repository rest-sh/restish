package output

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

type fakeFormatterStream struct {
	sendErr  error
	closeErr error
	closed   bool
}

func (s *fakeFormatterStream) Send(any) error {
	return s.sendErr
}

func (s *fakeFormatterStream) Close() error {
	s.closed = true
	return s.closeErr
}

func TestPluginFormatterStreamCloseClosesAfterFinalSendFailure(t *testing.T) {
	sendErr := errors.New("send failed")
	closeErr := errors.New("close failed")
	fake := &fakeFormatterStream{sendErr: sendErr, closeErr: closeErr}
	stream := &pluginFormatterStream{formatName: "csv", stream: fake}

	err := stream.Close()
	if err == nil {
		t.Fatal("expected close error")
	}
	if !fake.closed {
		t.Fatal("underlying stream was not closed after send failure")
	}
	if !errors.Is(err, sendErr) {
		t.Fatalf("expected joined send error, got %v", err)
	}
	if !errors.Is(err, closeErr) {
		t.Fatalf("expected joined close error, got %v", err)
	}
	if !strings.Contains(err.Error(), "formatter plugin csv") {
		t.Fatalf("expected formatter context, got %v", err)
	}
}

func TestPluginFormatterFormatJoinsFinalSendAndCloseErrors(t *testing.T) {
	sendErr := errors.New("send failed")
	closeErr := errors.New("close failed")
	fake := &fakeFormatterStream{sendErr: sendErr, closeErr: closeErr}
	oldStart := startPluginFormatterStream
	startPluginFormatterStream = func(ctx context.Context, path string, w io.Writer, in any) (formatterStream, error) {
		return fake, nil
	}
	t.Cleanup(func() { startPluginFormatterStream = oldStart })

	err := (&PluginFormatter{FormatName: "csv"}).Format(&bytes.Buffer{}, &Response{}, false)
	if err == nil {
		t.Fatal("expected format error")
	}
	if !fake.closed {
		t.Fatal("underlying stream was not closed")
	}
	if !errors.Is(err, sendErr) || !errors.Is(err, closeErr) {
		t.Fatalf("expected joined send and close errors, got %v", err)
	}
}

func TestPluginFormatterFormatValueJoinsItemSendAndCloseErrors(t *testing.T) {
	sendErr := errors.New("send failed")
	closeErr := errors.New("close failed")
	fake := &fakeFormatterStream{sendErr: sendErr, closeErr: closeErr}
	oldStart := startPluginFormatterStream
	startPluginFormatterStream = func(ctx context.Context, path string, w io.Writer, in any) (formatterStream, error) {
		return fake, nil
	}
	t.Cleanup(func() { startPluginFormatterStream = oldStart })

	err := (&PluginFormatter{FormatName: "csv"}).FormatValue(&bytes.Buffer{}, map[string]any{"id": 1}, false)
	if err == nil {
		t.Fatal("expected format value error")
	}
	if !fake.closed {
		t.Fatal("underlying stream was not closed")
	}
	if !errors.Is(err, sendErr) || !errors.Is(err, closeErr) {
		t.Fatalf("expected joined send and close errors, got %v", err)
	}
}
