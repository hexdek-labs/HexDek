package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// defaultServeAddr is what --serve binds to if --serve-addr is empty.
// Loopback-only by default — the coverage report has no auth and
// exposes whatever oracle data the user happens to have on disk; an
// open bind risks publishing that to the local network without
// consent. Users who want LAN access can set --serve-addr=:8765.
const defaultServeAddr = "127.0.0.1:8765"

// htmlHandler is the single-route HTTP handler used by --serve. The
// root path returns the rendered HTML bytes; any other path 404s.
// Pulled out so tests can exercise the handler directly through
// httptest without spinning up a real listener.
func htmlHandler(html []byte) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(html)
	})
	return mux
}

// resolveAddrForBrowser turns the bind address into a user-clickable
// URL host:port. Bare ":8765" reads as wildcard-binding which would
// confuse a user pasting it into a browser, so we substitute
// localhost. Anything else is returned verbatim.
func resolveAddrForBrowser(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "localhost" + addr
	}
	return addr
}

// serveHTML runs an HTTP server bound to addr that serves the given
// HTML bytes at /. It blocks until ctx is cancelled (e.g., by SIGINT
// wiring in main.go) or the underlying listener errors.
//
// Bind happens synchronously before returning to the goroutine so a
// port-in-use error surfaces immediately, not asynchronously while
// the user is browsing. The actual listening address is reported via
// the callback so the caller can log the resolved URL (useful when
// addr is ":0" for tests or for picking a free port).
func serveHTML(ctx context.Context, addr string, html []byte, onListening func(actualAddr string)) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	srv := &http.Server{
		Handler:           htmlHandler(html),
		ReadHeaderTimeout: 5 * time.Second,
	}
	if onListening != nil {
		onListening(ln.Addr().String())
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		// Graceful shutdown — give in-flight requests a moment to
		// finish but don't wait forever if the client hangs.
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		// Drain the Serve goroutine so the test isn't racy.
		<-errCh
		return nil
	case err := <-errCh:
		return err
	}
}
