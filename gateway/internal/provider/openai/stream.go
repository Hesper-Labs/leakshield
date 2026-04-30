package openai

import (
	"io"
	"net/http"

	"github.com/Hesper-Labs/leakshield/gateway/internal/provider"
)

// newSSEReader returns the shared SSE reader implementation. Wrapped here
// to keep the provider package import surface small and to leave room for
// OpenAI-specific finish-reason / token-usage parsing later without
// touching the shared reader.
func newSSEReader(body io.ReadCloser, headers http.Header) provider.StreamReader {
	return provider.NewSSEReader(body, headers)
}
