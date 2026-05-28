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
	connectTimeout = 10 * time.Second
	readTimeout    = 30 * time.Second
	maxBodyBytes   = 1 << 20 // 1 MiB
)

// Query performs a single finger query against t.HostPort.
// On read deadline or size cap, returns the partial body with
// Meta.Truncated = true and a non-nil err.
func Query(ctx context.Context, t Target) ([]byte, Meta, error) {
	start := time.Now()
	meta := Meta{Addr: t.HostPort}

	dialCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(dialCtx, "tcp", t.HostPort)
	if err != nil {
		meta.Elapsed = time.Since(start)
		return nil, meta, fmt.Errorf("dial %s: %w", t.HostPort, err)
	}
	defer conn.Close()

	// Send the query line: "<user>\r\n" (user may be empty for @host).
	if _, err := fmt.Fprintf(conn, "%s\r\n", t.User); err != nil {
		meta.Elapsed = time.Since(start)
		return nil, meta, fmt.Errorf("write query: %w", err)
	}

	// Read until EOF.
	raw, err := io.ReadAll(conn)
	if err != nil {
		meta.Elapsed = time.Since(start)
		return nil, meta, fmt.Errorf("read response: %w", err)
	}

	body := bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	meta.Bytes = len(body)
	meta.Elapsed = time.Since(start)
	return body, meta, nil
}
