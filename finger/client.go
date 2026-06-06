package finger

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"time"
	"unicode/utf8"
)

// Meta describes the outcome of a Query that produced a body.
type Meta struct {
	Addr      string
	Elapsed   time.Duration
	Bytes     int
	Truncated bool
}

const (
	defaultConnectTimeout = 10 * time.Second
	defaultReadTimeout    = 30 * time.Second
	maxBodyBytes          = 1 << 20 // 1 MiB
)

// queryOpts are tuneable knobs used by tests. Zero values mean "use defaults".
type queryOpts struct {
	connectTimeout time.Duration
	readTimeout    time.Duration
}

// queryWith is Query with overrides. The exported Query forwards with zero opts.
func queryWith(ctx context.Context, t Target, opts queryOpts) ([]byte, Meta, error) {
	if opts.connectTimeout == 0 {
		opts.connectTimeout = defaultConnectTimeout
	}
	if opts.readTimeout == 0 {
		opts.readTimeout = defaultReadTimeout
	}

	start := time.Now()
	meta := Meta{Addr: t.HostPort}

	dialCtx, cancel := context.WithTimeout(ctx, opts.connectTimeout)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(dialCtx, "tcp", t.HostPort)
	if err != nil {
		meta.Elapsed = time.Since(start)
		return nil, meta, fmt.Errorf("dial %s: %w", t.HostPort, err)
	}
	defer conn.Close()

	// Propagate caller-ctx cancellation to the connection. A blocking
	// net.Conn.Read does not observe context.Done() on its own.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()

	// Overall read deadline. context.WithTimeout alone does NOT interrupt
	// a blocking net.Conn.Read; SetDeadline does.
	if err := conn.SetDeadline(time.Now().Add(opts.readTimeout)); err != nil {
		meta.Elapsed = time.Since(start)
		return nil, meta, fmt.Errorf("set deadline: %w", err)
	}

	query := t.QueryLine()
	if hasControl(query) {
		meta.Elapsed = time.Since(start)
		return nil, meta, fmt.Errorf("query contains control characters")
	}
	if _, err := fmt.Fprintf(conn, "%s\r\n", query); err != nil {
		meta.Elapsed = time.Since(start)
		return nil, meta, fmt.Errorf("write query: %w", err)
	}

	// Read up to maxBodyBytes + 1 so we can detect overflow.
	lr := &io.LimitedReader{R: conn, N: maxBodyBytes + 1}
	raw, readErr := io.ReadAll(lr)

	truncatedByCap := false
	if len(raw) > maxBodyBytes {
		raw = raw[:maxBodyBytes]
		truncatedByCap = true
	}

	body := bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	body = sanitize(body)
	if cappedBody, ok := capBody(body); ok {
		body = cappedBody
		truncatedByCap = true
	}
	meta.Bytes = len(body)
	meta.Elapsed = time.Since(start)

	if truncatedByCap {
		meta.Truncated = true
		// readErr may be non-nil (timeout) or nil — either way, cap wins.
		return body, meta, nil
	}

	if readErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return body, meta, ctxErr
		}
		// Timeout? Treat as truncated. Other errors propagate as-is.
		if ne, ok := readErr.(net.Error); ok && ne.Timeout() {
			meta.Truncated = true
			return body, meta, fmt.Errorf("read response timed out after %s: %w", opts.readTimeout, readErr)
		}
		if len(body) > 0 {
			// Finger servers commonly reset the connection right after sending
			// their body instead of closing cleanly. We can't be certain the
			// body was complete, but one cut mid-line (no trailing newline) was
			// almost certainly truncated; a newline-terminated body is treated
			// as complete to avoid false "truncated" flags on the common
			// reset-after-body case.
			if body[len(body)-1] != '\n' {
				meta.Truncated = true
			}
			return body, meta, nil
		}
		return body, meta, fmt.Errorf("read response: %w", readErr)
	}
	return body, meta, nil
}

func Query(ctx context.Context, t Target) ([]byte, Meta, error) {
	return queryWith(ctx, t, queryOpts{})
}

func capBody(body []byte) ([]byte, bool) {
	if len(body) <= maxBodyBytes {
		return body, false
	}
	body = body[:maxBodyBytes]
	for !utf8.Valid(body) {
		body = body[:len(body)-1]
	}
	return body, true
}
