package main

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/rest-sh/restish/v2/plugin"
)

func TestPluginClientDoReturnsEOFWhenHostReadLoopEnds(t *testing.T) {
	client := newPluginClient(plugin.NewDecoder(bytes.NewReader(nil)), &bytes.Buffer{})
	go client.readLoop()

	done := make(chan error, 1)
	go func() {
		_, err := client.do(&HTTPRequest{Method: "GET", URI: "https://example.com"})
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, io.EOF) {
			t.Fatalf("expected io.EOF, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for do to return")
	}
}

func TestPluginClientCorrelatesHTTPResponsesByRequestID(t *testing.T) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	client := newPluginClient(plugin.NewDecoder(inR), outW)

	var wg sync.WaitGroup
	results := make([]*HTTPResponse, 2)
	errs := make([]error, 2)
	var resultsMu sync.Mutex

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp, err := client.do(&HTTPRequest{Method: "GET", URI: "https://example.com/" + strconv.Itoa(i+1)})
			resultsMu.Lock()
			results[i] = resp
			errs[i] = err
			resultsMu.Unlock()
		}(i)
	}

	gotRequests := make([]plugin.HTTPRequestMsg, 0, 2)
	outDec := plugin.NewDecoder(outR)
	for len(gotRequests) < 2 {
		var req plugin.HTTPRequestMsg
		if err := outDec.ReadMessage(&req); err != nil {
			t.Fatalf("read request %d: %v", len(gotRequests)+1, err)
		}
		gotRequests = append(gotRequests, req)
	}

	go client.readLoop()

	for i := len(gotRequests) - 1; i >= 0; i-- {
		req := gotRequests[i]
		status := 0
		switch req.URI {
		case "https://example.com/1":
			status = 201
		case "https://example.com/2":
			status = 202
		default:
			t.Fatalf("unexpected request URI %q", req.URI)
		}
		if err := plugin.WriteMessage(inW, plugin.HTTPResponseMsg{
			Type:      plugin.MsgTypeHTTPResponse,
			RequestID: req.RequestID,
			Status:    status,
			Body:      map[string]any{"uri": req.URI},
		}); err != nil {
			t.Fatal(err)
		}
	}
	_ = inW.Close()

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
	}
	if results[0].Status != 201 {
		t.Fatalf("request 0 got status %d, want 201", results[0].Status)
	}
	if results[1].Status != 202 {
		t.Fatalf("request 1 got status %d, want 202", results[1].Status)
	}
}
