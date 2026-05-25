package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHTMLHandler_ServesBytesAtRoot(t *testing.T) {
	body := []byte("<html><body>hello</body></html>")
	srv := httptest.NewServer(htmlHandler(body))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: want 200, got %d", resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	if string(got) != string(body) {
		t.Errorf("body mismatch: got %q", got)
	}
}

func TestHTMLHandler_ContentTypeHTML(t *testing.T) {
	srv := httptest.NewServer(htmlHandler([]byte("<html></html>")))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type: want text/html prefix, got %q", ct)
	}
	if !strings.Contains(ct, "utf-8") {
		t.Errorf("Content-Type should declare utf-8 charset: %q", ct)
	}
}

func TestHTMLHandler_NotFoundForOtherPaths(t *testing.T) {
	srv := httptest.NewServer(htmlHandler([]byte("hello")))
	defer srv.Close()
	for _, path := range []string{"/index.html", "/api", "/some/sub/path"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: want 404, got %d", path, resp.StatusCode)
		}
	}
}

func TestHTMLHandler_EmptyBodyIsOK(t *testing.T) {
	// An empty payload is still a valid 200 — no special-casing.
	srv := httptest.NewServer(htmlHandler(nil))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("empty body: want 200, got %d", resp.StatusCode)
	}
}

func TestResolveAddrForBrowser_BareColonGetsLocalhost(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{":8765", "localhost:8765"},
		{":0", "localhost:0"},
		{"127.0.0.1:8765", "127.0.0.1:8765"},
		{"192.168.1.10:8765", "192.168.1.10:8765"},
		{"localhost:9000", "localhost:9000"},
	}
	for _, c := range cases {
		if got := resolveAddrForBrowser(c.in); got != c.want {
			t.Errorf("resolveAddrForBrowser(%q): got %q, want %q", c.in, got, c.want)
		}
	}
}

func TestServeHTML_EndToEndServesAndShutsDownOnContextCancel(t *testing.T) {
	html := []byte("<html><body>e2e</body></html>")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addrCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- serveHTML(ctx, "127.0.0.1:0", html, func(addr string) { addrCh <- addr })
	}()

	var addr string
	select {
	case addr = <-addrCh:
	case <-time.After(2 * time.Second):
		t.Fatal("serveHTML did not start within 2s")
	}

	// Verify the actual port is reachable and returns our body.
	resp, err := http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	got, _ := io.ReadAll(resp.Body)
	if string(got) != string(html) {
		t.Errorf("body: got %q, want %q", got, html)
	}

	// Cancel the context — serveHTML should shut down promptly.
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("serveHTML returned error on shutdown: %v", err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("serveHTML did not return within 6s of ctx cancellation")
	}
}

func TestServeHTML_PortInUseReturnsImmediateError(t *testing.T) {
	// Grab an ephemeral port, hold it, then ask serveHTML to bind to
	// the same address. The Listen call inside serveHTML must fail
	// synchronously rather than starting the server and dying later.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	addr := ln.Addr().String()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- serveHTML(ctx, addr, []byte("x"), nil)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error when binding to already-bound address, got nil")
		}
		if !strings.Contains(err.Error(), "listen") {
			t.Errorf("error should mention listen: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serveHTML should have errored immediately on bind failure, but didn't return")
	}
}

func TestServeHTML_ConcurrentRequestsAllSucceed(t *testing.T) {
	// Smoke-test that the handler tolerates parallel hits (defense
	// against accidentally introducing a mutex around the body bytes
	// or other handler-level state).
	html := []byte(strings.Repeat("a", 1024))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addrCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() { errCh <- serveHTML(ctx, "127.0.0.1:0", html, func(a string) { addrCh <- a }) }()
	addr := <-addrCh

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := http.Get("http://" + addr + "/")
			if err != nil {
				t.Errorf("concurrent GET: %v", err)
				return
			}
			defer resp.Body.Close()
			b, _ := io.ReadAll(resp.Body)
			if len(b) != len(html) {
				t.Errorf("concurrent GET body length: want %d, got %d", len(html), len(b))
			}
		}()
	}
	wg.Wait()
	cancel()
	<-errCh
}

func TestRenderHTMLReport_ReturnsSameBytesAsWriteHTMLReport(t *testing.T) {
	// The refactor introduced renderHTMLReport (in-memory) +
	// writeHTMLReport (wrapper). Both should produce byte-identical
	// output for the same input — verifies the wrapper isn't
	// re-rendering or mutating along the way.
	rs := fixture()
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	mem, err := renderHTMLReport(rs, false, now)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	path := dir + "/out.html"
	if err := writeHTMLReport(path, rs, false, now); err != nil {
		t.Fatal(err)
	}
	disk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(mem) != string(disk) {
		t.Error("renderHTMLReport bytes don't match writeHTMLReport on-disk output")
	}
}
