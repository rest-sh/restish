package output

import (
	"errors"
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
