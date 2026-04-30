// Package provider defines the upstream LLM adapter contract used by the
// gateway. Each adapter is a thin pass-through proxy for one provider's
// native API: it parses just enough of the request body to extract messages
// for DLP inspection, swaps the auth header for the company's stored master
// key, and forwards everything else byte-for-byte.
//
// The pass-through design is the central architectural decision (see
// docs/architecture.md): we deliberately do NOT translate between provider
// schemas. Each provider gets its own URL prefix and clients use the
// provider's native SDK with only a base_url change.
package provider

import (
	"context"
	"errors"
	"io"
	"net/http"
)

// Errors returned by the package.
var (
	// ErrUnsupportedRoute is returned when an adapter is asked to handle
	// a path it does not serve.
	ErrUnsupportedRoute = errors.New("provider: unsupported route")

	// ErrInvalidBody is returned when the body cannot be parsed enough to
	// extract messages for inspection.
	ErrInvalidBody = errors.New("provider: invalid body")

	// ErrUpstream wraps any non-2xx (and non-streaming-related) failure
	// from the upstream provider.
	ErrUpstream = errors.New("provider: upstream error")
)

// Route describes a single URL pattern this provider serves.
//
// Patterns use chi-style placeholders (e.g. "/openai/v1/chat/completions"
// or "/google/v1beta/models/{model}:generateContent").
type Route struct {
	Method  string
	Pattern string
}

// PassthroughRequest carries the raw bytes the client sent us, plus the
// metadata an adapter needs to issue the upstream call. The body is held
// in memory so adapters can both inspect it and forward it; the gateway
// caps body size upstream of the adapter.
type PassthroughRequest struct {
	// Method is the HTTP method as received from the client (POST/GET).
	Method string

	// Path is the URL path the client called (without host). The provider
	// adapter rewrites this into the upstream URL.
	Path string

	// Query is the raw query string (without leading '?'). Used by Google
	// (api key) and Azure (api-version) and forwarded by every adapter.
	Query string

	// Headers are the client-provided headers, with hop-by-hop and
	// auth-bearing headers already stripped by the gateway. Adapters add
	// their auth header(s) before forwarding.
	Headers http.Header

	// Body is the raw request body bytes (already buffered). May be nil
	// for GET requests. Adapters MUST forward this byte-for-byte unless
	// the DLP verdict requires masking, in which case the gateway calls
	// InjectMessages to rewrite the affected fields and updates Body in
	// place before calling Forward / Stream.
	Body []byte
}

// PassthroughResponse holds an upstream non-streaming response captured by
// the adapter. The gateway copies Status/Headers/Body to the client.
type PassthroughResponse struct {
	Status  int
	Headers http.Header
	Body    []byte
}

// StreamReader yields SSE chunks from the upstream provider. The chunks
// are emitted in the provider's own dialect (OpenAI's data: lines,
// Anthropic's event:/data: pairs, Google's JSON-array streaming, etc.) and
// the gateway forwards them to the client unchanged.
//
// Implementations must be safe to call Close on more than once.
type StreamReader interface {
	// Next returns the next chunk. It returns io.EOF when the stream is
	// exhausted. Any other error is returned as-is and the stream is
	// closed.
	Next(ctx context.Context) (*StreamChunk, error)
	// Close releases the underlying HTTP body and any related resources.
	Close() error
}

// StreamChunk is one SSE event from the upstream. Raw is the bytes the
// gateway should write to the client (including the trailing blank line
// SSE expects). The optional fields let an adapter surface metadata for
// auditing / token accounting without re-parsing the byte slice.
type StreamChunk struct {
	Raw          []byte
	FinishReason string // "stop", "length", "content_filter", ...
	UsageTokens  int    // 0 if unknown / not yet reported
}

// Message is the normalized message tuple the inspector consumes. It maps
// 1:1 to leakshield.inspector.v1.Message (see proto/inspector/v1/inspector.proto).
//
// Roles are normalized to the inspector's vocabulary:
//
//	system | user | assistant | tool
//
// Adapter-specific role names (e.g. Google's "model") are translated to
// these by ExtractMessages and translated back by InjectMessages.
type Message struct {
	Role    string
	Content string
}

// ProviderKey is the resolved master credential for one (tenant, provider)
// pair. The Master string is the plaintext API key after envelope
// decryption; never log it. Extra is provider-specific configuration —
// for OpenAI it is empty; for Azure it carries the resource endpoint and
// the per-model deployment map; for Google it can carry a project / region.
type ProviderKey struct {
	Master string
	Extra  ProviderKeyExtra
}

// ProviderKeyExtra is a typed bag for provider-specific config. Each
// adapter only reads the fields relevant to it.
type ProviderKeyExtra struct {
	// Azure-specific.
	AzureEndpoint    string            // e.g. "https://my-resource.openai.azure.com"
	AzureDeployments map[string]string // model name -> deployment name
	AzureAPIVersion  string            // default api-version if the request omits one

	// OpenAI-specific.
	OpenAIOrgID string // optional, sent as OpenAI-Organization header

	// Google-specific.
	GoogleProject string // reserved for Vertex AI; unused by the public API adapter
}

// Provider represents one upstream LLM API. Adapters implement
// pass-through forwarding plus enough body parsing to pull messages out
// for DLP inspection.
type Provider interface {
	// Name returns the canonical provider name: "openai", "anthropic",
	// "google", "azure".
	Name() string

	// Routes returns the URL patterns this provider serves. The gateway
	// uses the URL prefix (the first segment) to dispatch incoming
	// requests; the full pattern is informational.
	Routes() []Route

	// SupportedModels returns the model identifiers this adapter knows
	// about. Used by the catalog and the policy editor's allowlist
	// defaults; not authoritative — clients can always pass an unknown
	// model and the upstream will validate it.
	SupportedModels() []string

	// Forward issues a non-streaming upstream call. The adapter MUST pass
	// through req.Body byte-for-byte except for swapping the company's
	// master key into the auth header(s).
	Forward(ctx context.Context, req *PassthroughRequest, key *ProviderKey) (*PassthroughResponse, error)

	// Stream issues an SSE-streamed upstream call. The returned
	// StreamReader emits chunks in the provider's own SSE dialect; the
	// gateway forwards them to the client unchanged.
	Stream(ctx context.Context, req *PassthroughRequest, key *ProviderKey) (StreamReader, error)

	// ExtractMessages parses the request body well enough to give the
	// inspector a normalized list of messages. Adapter-specific quirks
	// (Anthropic's top-level system field, Google's contents/parts) live
	// here.
	ExtractMessages(req *PassthroughRequest) ([]Message, error)

	// InjectMessages serializes masked messages back into the body. Used
	// when the DLP verdict is MASK. The adapter MUST preserve every other
	// field of the original body.
	InjectMessages(req *PassthroughRequest, masked []Message) error

	// CountTokens returns a pre-flight token estimate (best-effort). It
	// is used for rate limiting and for checking input token caps; it is
	// not a billing source of truth.
	CountTokens(req *PassthroughRequest) (int, error)
}

// CopyHeaders is a small helper used by adapters to clone an http.Header
// without sharing the underlying slice memory.
func CopyHeaders(src http.Header) http.Header {
	dst := make(http.Header, len(src))
	for k, vs := range src {
		copied := make([]string, len(vs))
		copy(copied, vs)
		dst[k] = copied
	}
	return dst
}

// DrainAndClose reads any remaining bytes from r and closes it. Adapters
// use this to free up keep-alive connections when they only care about a
// non-success status code.
func DrainAndClose(r io.ReadCloser) {
	if r == nil {
		return
	}
	_, _ = io.Copy(io.Discard, r)
	_ = r.Close()
}
