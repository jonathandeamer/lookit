package finger

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"time"
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

	// Overall read deadline. context.WithTimeout alone does NOT interrupt
	// a blocking net.Conn.Read; SetDeadline does.
	if err := conn.SetDeadline(time.Now().Add(opts.readTimeout)); err != nil {
		meta.Elapsed = time.Since(start)
		return nil, meta, fmt.Errorf("set deadline: %w", err)
	}

	if _, err := fmt.Fprintf(conn, "%s\r\n", t.User); err != nil {
		meta.Elapsed = time.Since(start)
		return nil, meta, fmt.Errorf("write query: %w", err)
	}

	raw, readErr := io.ReadAll(conn)
	body := bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	meta.Bytes = len(body)
	meta.Elapsed = time.Since(start)

	if readErr != nil {
		// Timeout? Treat as truncated. Other errors propagate as-is.
		if ne, ok := readErr.(net.Error); ok && ne.Timeout() {
			meta.Truncated = true
			return body, meta, fmt.Errorf("read response timed out after %s: %w", opts.readTimeout, readErr)
		}
		return body, meta, fmt.Errorf("read response: %w", readErr)
	}
	return body, meta, nil
}

func Query(ctx context.Context, t Target) ([]byte, Meta, error) {
	return queryWith(ctx, t, queryOpts{})
}
