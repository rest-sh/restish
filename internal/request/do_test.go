package request_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rest-sh/restish/v2/internal/request"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestDo_BasicGet(t *testing.T) {
	resp, err := request.Do(context.Background(), "GET", "https://api.example.com/items", nil, request.Options{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != "GET" {
				t.Errorf("expected GET, got %s", r.Method)
			}
			return response(200, "hello"), nil
		}),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello" {
		t.Errorf("expected body 'hello', got %q", body)
	}
}

func TestDo_Headers(t *testing.T) {
	var gotHeader string
	_, err := request.Do(context.Background(), "GET", "https://api.example.com/items", nil, request.Options{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			gotHeader = r.Header.Get("X-Custom")
			return response(200, ""), nil
		}),
		Headers: []string{"X-Custom: test-value"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotHeader != "test-value" {
		t.Errorf("expected X-Custom=test-value, got %q", gotHeader)
	}
}

func TestDo_HeaderWithColonInValue(t *testing.T) {
	var gotHeader string
	_, err := request.Do(context.Background(), "GET", "https://api.example.com/items", nil, request.Options{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			gotHeader = r.Header.Get("Authorization")
			return response(200, ""), nil
		}),
		Headers: []string{"Authorization: Bearer tok:en"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotHeader != "Bearer tok:en" {
		t.Errorf("expected 'Bearer tok:en', got %q", gotHeader)
	}
}

func TestDo_HostHeaderSetsRequestHost(t *testing.T) {
	var gotHost, gotHeader string
	_, err := request.Do(context.Background(), "GET", "https://api.example.com/items", nil, request.Options{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			gotHost = r.Host
			gotHeader = r.Header.Get("Host")
			return response(200, ""), nil
		}),
		Headers: []string{"Host: tenant.example.com"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotHost != "tenant.example.com" {
		t.Errorf("expected req.Host tenant.example.com, got %q", gotHost)
	}
	if gotHeader != "" {
		t.Errorf("expected Host not to be stored as a regular header, got %q", gotHeader)
	}
}

func TestDo_InvalidHeader(t *testing.T) {
	_, err := request.Do(context.Background(), "GET", "https://api.example.com/items", nil, request.Options{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return response(200, ""), nil
		}),
		Headers: []string{"no-colon-at-all"},
	})
	if err == nil {
		t.Fatal("expected error for malformed header, got nil")
	}
}

func TestDo_Query(t *testing.T) {
	var gotQuery string
	_, err := request.Do(context.Background(), "GET", "https://api.example.com/items", nil, request.Options{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			gotQuery = r.URL.Query().Get("page")
			return response(200, ""), nil
		}),
		Query: []string{"page=2"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotQuery != "2" {
		t.Errorf("expected page=2, got %q", gotQuery)
	}
}

func TestDo_InvalidQueryParam(t *testing.T) {
	_, err := request.Do(context.Background(), "GET", "https://api.example.com/items", nil, request.Options{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return response(200, ""), nil
		}),
		Query: []string{"no-equals-sign"},
	})
	if err == nil {
		t.Fatal("expected error for malformed query param, got nil")
	}
}

func TestDo_Post_WithBody(t *testing.T) {
	var gotBody string
	resp, err := request.Do(context.Background(), "POST", "https://api.example.com/items", strings.NewReader(`{"name":"test"}`), request.Options{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			b, _ := io.ReadAll(r.Body)
			gotBody = string(b)
			return response(201, ""), nil
		}),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
	if gotBody != `{"name":"test"}` {
		t.Errorf("unexpected body: %q", gotBody)
	}
}

func TestDo_CrossOriginRedirectStripsCredentialHeaders(t *testing.T) {
	var gotAPIKey, gotTrace string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-API-Key")
		gotTrace = r.Header.Get("X-Trace")
		w.WriteHeader(200)
	}))
	defer target.Close()

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/next", http.StatusFound)
	}))
	defer source.Close()

	resp, err := request.Do(context.Background(), "GET", source.URL, nil, request.Options{
		Headers: []string{"X-API-Key: secret", "X-Trace: keep"},
	})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()
	if gotAPIKey != "" {
		t.Fatalf("X-API-Key crossed origin: %q", gotAPIKey)
	}
	if gotTrace != "keep" {
		t.Fatalf("X-Trace = %q, want keep", gotTrace)
	}
}

func TestDo_SameOriginRedirectKeepsCredentialHeaders(t *testing.T) {
	var gotAPIKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/next", http.StatusFound)
			return
		}
		gotAPIKey = r.Header.Get("X-API-Key")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	resp, err := request.Do(context.Background(), "GET", srv.URL+"/start", nil, request.Options{
		Headers: []string{"X-API-Key: secret"},
	})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()
	if gotAPIKey != "secret" {
		t.Fatalf("X-API-Key = %q, want secret", gotAPIKey)
	}
}

func TestDo_HeaderTimeout(t *testing.T) {
	_, err := request.Do(context.Background(), "GET", "https://api.example.com/items", nil, request.Options{
		Timeout: 20 * time.Millisecond,
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			<-r.Context().Done()
			return nil, r.Context().Err()
		}),
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if err != context.DeadlineExceeded {
		t.Fatalf("err = %v, want %v", err, context.DeadlineExceeded)
	}
}

func TestDo_HeaderTimeoutDoesNotWaitForNonCooperativeTransport(t *testing.T) {
	release := make(chan struct{})
	closed := make(chan struct{})
	started := make(chan struct{})

	start := time.Now()
	_, err := request.Do(context.Background(), "GET", "https://api.example.com/items", nil, request.Options{
		Timeout: 20 * time.Millisecond,
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			close(started)
			<-release
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{},
				Body:       closeNotifyBody{closed: closed},
			}, nil
		}),
	})
	elapsed := time.Since(start)
	if err != context.DeadlineExceeded {
		t.Fatalf("err = %v, want %v", err, context.DeadlineExceeded)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("timeout waited for non-cooperative transport, elapsed = %v", elapsed)
	}
	<-started
	close(release)
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("late response body was not closed")
	}
}

func TestDo_TimeoutCancelsBodyReads(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resp, err := request.Do(ctx, "GET", "https://api.example.com/items", nil, request.Options{
		Timeout: 20 * time.Millisecond,
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{},
				Body: io.NopCloser(readerFunc(func(p []byte) (int, error) {
					select {
					case <-time.After(50 * time.Millisecond):
						copy(p, "hello")
						return 5, io.EOF
					case <-r.Context().Done():
						return 0, r.Context().Err()
					}
				})),
			}, nil
		}),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	_, err = io.ReadAll(resp.Body)
	if err == nil {
		t.Fatal("expected body read timeout")
	}
	if err != context.DeadlineExceeded {
		t.Fatalf("body read error = %v, want %v", err, context.DeadlineExceeded)
	}
}

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) {
	return f(p)
}

type closeNotifyBody struct {
	closed chan struct{}
}

func (b closeNotifyBody) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (b closeNotifyBody) Close() error {
	close(b.closed)
	return nil
}
