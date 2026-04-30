package provider

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
)

// SSEReader is a basic StreamReader implementation that emits SSE events
// (delimited by a blank line) from any io.ReadCloser. Several adapters
// (OpenAI, Anthropic, Azure) share the same wire format.
//
// SSEReader does NOT mutate the bytes — each emitted chunk is the exact
// event including the trailing blank line, suitable for direct write to
// the client.
type SSEReader struct {
	r       io.ReadCloser
	headers http.Header
	br      *bufio.Reader
	buf     bytes.Buffer
	closed  bool
}

// NewSSEReader wraps an upstream HTTP body for streaming pass-through.
func NewSSEReader(body io.ReadCloser, headers http.Header) *SSEReader {
	return &SSEReader{
		r:       body,
		headers: headers,
		br:      bufio.NewReaderSize(body, 64*1024),
	}
}

// Headers returns the upstream response headers (Content-Type etc.) so
// the gateway can echo them to the client.
func (s *SSEReader) Headers() http.Header { return s.headers }

// Next reads the next SSE event. It returns io.EOF at end of stream.
// Empty (heartbeat) lines are coalesced into the next data event so the
// caller sees one chunk per upstream event.
func (s *SSEReader) Next(ctx context.Context) (*StreamChunk, error) {
	if s.closed {
		return nil, io.EOF
	}
	// Honor context cancellation cheaply between reads. The underlying
	// HTTP connection is bound to the request context, so a real cancel
	// will trip ReadString below regardless.
	if ctx != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	s.buf.Reset()
	for {
		line, err := s.br.ReadBytes('\n')
		if len(line) > 0 {
			s.buf.Write(line)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				if s.buf.Len() == 0 {
					return nil, io.EOF
				}
				out := make([]byte, s.buf.Len())
				copy(out, s.buf.Bytes())
				return &StreamChunk{Raw: out}, nil
			}
			return nil, err
		}
		// SSE delimits events with a blank line.
		if isBlankLine(line) && s.buf.Len() > len(line) {
			out := make([]byte, s.buf.Len())
			copy(out, s.buf.Bytes())
			return &StreamChunk{Raw: out}, nil
		}
	}
}

// Close releases the underlying body. Safe to call multiple times.
func (s *SSEReader) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	if s.r != nil {
		return s.r.Close()
	}
	return nil
}

func isBlankLine(b []byte) bool {
	switch len(b) {
	case 1:
		return b[0] == '\n'
	case 2:
		return b[0] == '\r' && b[1] == '\n'
	}
	return false
}
