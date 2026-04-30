// Package openai implements the OpenAI pass-through adapter.
//
// The adapter forwards requests to api.openai.com unchanged except for
// swapping the inbound Authorization header with the company's master
// key. We parse the body just enough to extract messages for DLP
// inspection (the standard chat/completions schema) and to detect the
// stream flag.
//
// Supported endpoints:
//
//	POST /openai/v1/chat/completions
//	POST /openai/v1/embeddings
//	POST /openai/v1/responses
//	GET  /openai/v1/models
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Hesper-Labs/leakshield/gateway/internal/provider"
)

// upstreamBase is the production OpenAI API base. The trailing /v1 is
// included so the adapter can simply append the path under /openai/v1/...
// (after stripping the /openai prefix) and arrive at the correct URL.
const upstreamBase = "https://api.openai.com"

// Provider is the OpenAI adapter.
type Provider struct {
	httpClient *http.Client
	baseURL    string
}

// New constructs an OpenAI adapter with a shared transport tuned for
// HTTP/2 keep-alive.
func New() *Provider {
	return &Provider{
		httpClient: newHTTPClient(),
		baseURL:    upstreamBase,
	}
}

// newWithBase is used by tests to point the adapter at httptest.Server.
func newWithBase(base string) *Provider {
	return &Provider{
		httpClient: newHTTPClient(),
		baseURL:    base,
	}
}

func newHTTPClient() *http.Client {
	t := &http.Transport{
		MaxIdleConnsPerHost: 256,
		IdleConnTimeout:     90 * time.Second,
		ForceAttemptHTTP2:   true,
	}
	return &http.Client{
		Transport: t,
		// No top-level timeout — streaming requests outlive any wall clock.
	}
}

func init() {
	provider.Register("openai", func() (provider.Provider, error) {
		p := New()
		if testBaseURL != "" {
			p.baseURL = testBaseURL
		}
		return p, nil
	})
}

// Name returns the canonical provider name.
func (p *Provider) Name() string { return "openai" }

// Routes lists the chi-style URL patterns this adapter serves.
func (p *Provider) Routes() []provider.Route {
	return []provider.Route{
		{Method: http.MethodPost, Pattern: "/openai/v1/chat/completions"},
		{Method: http.MethodPost, Pattern: "/openai/v1/embeddings"},
		{Method: http.MethodPost, Pattern: "/openai/v1/responses"},
		{Method: http.MethodGet, Pattern: "/openai/v1/models"},
	}
}

// SupportedModels returns common OpenAI model identifiers. Used as
// catalog defaults; not enforced as an allowlist.
func (p *Provider) SupportedModels() []string {
	return []string{
		"gpt-4o",
		"gpt-4o-mini",
		"gpt-4-turbo",
		"gpt-4",
		"gpt-3.5-turbo",
		"o1",
		"o1-mini",
		"text-embedding-3-small",
		"text-embedding-3-large",
		"text-embedding-ada-002",
	}
}

// chatRequest is the subset of the OpenAI chat/completions schema we
// need: messages for DLP, stream for routing, model for audit. We never
// re-marshal the full struct — we update messages in place via the
// generic JSON map below.
type chatMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
	Name    string          `json:"name,omitempty"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream,omitempty"`
}

// ExtractMessages parses an OpenAI chat/completions or responses body
// and returns the messages in normalized form. Embeddings and models
// requests have no messages and return an empty slice.
func (p *Provider) ExtractMessages(req *provider.PassthroughRequest) ([]provider.Message, error) {
	if !needsBodyParse(req) {
		return nil, nil
	}
	if len(req.Body) == 0 {
		return nil, nil
	}

	var cr chatRequest
	if err := json.Unmarshal(req.Body, &cr); err != nil {
		return nil, fmt.Errorf("%w: %v", provider.ErrInvalidBody, err)
	}

	out := make([]provider.Message, 0, len(cr.Messages))
	for _, m := range cr.Messages {
		out = append(out, provider.Message{
			Role:    m.Role,
			Content: openAIContentToString(m.Content),
		})
	}
	return out, nil
}

// openAIContentToString flattens an OpenAI chat content field, which may
// be a string or an array of content parts ({type: "text", text: "..."}
// or {type: "image_url", image_url: ...}). We extract text only; images
// pass through untouched in the wire body.
func openAIContentToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Fast path: plain string.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}

	var parts []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &parts); err != nil {
		return ""
	}
	var b strings.Builder
	for _, part := range parts {
		typeRaw, ok := part["type"]
		if !ok {
			continue
		}
		var ptype string
		if err := json.Unmarshal(typeRaw, &ptype); err != nil {
			continue
		}
		if ptype == "text" {
			textRaw, ok := part["text"]
			if !ok {
				continue
			}
			var text string
			if err := json.Unmarshal(textRaw, &text); err == nil {
				if b.Len() > 0 {
					b.WriteString("\n")
				}
				b.WriteString(text)
			}
		}
	}
	return b.String()
}

// InjectMessages writes the masked messages back into req.Body, leaving
// every other field of the original body untouched.
func (p *Provider) InjectMessages(req *provider.PassthroughRequest, masked []provider.Message) error {
	if !needsBodyParse(req) {
		return nil
	}
	if len(req.Body) == 0 {
		return nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(req.Body, &raw); err != nil {
		return fmt.Errorf("%w: %v", provider.ErrInvalidBody, err)
	}
	rawMessages, ok := raw["messages"]
	if !ok {
		// Not a chat request (e.g. embeddings) — nothing to mask.
		return nil
	}

	var orig []map[string]json.RawMessage
	if err := json.Unmarshal(rawMessages, &orig); err != nil {
		return fmt.Errorf("%w: %v", provider.ErrInvalidBody, err)
	}
	if len(orig) != len(masked) {
		return fmt.Errorf("%w: masked count %d != original %d", provider.ErrInvalidBody, len(masked), len(orig))
	}
	for i := range orig {
		contentJSON, err := json.Marshal(masked[i].Content)
		if err != nil {
			return err
		}
		orig[i]["content"] = contentJSON
		// Role too, in case the inspector normalizes it.
		roleJSON, err := json.Marshal(masked[i].Role)
		if err != nil {
			return err
		}
		orig[i]["role"] = roleJSON
	}
	newMessages, err := json.Marshal(orig)
	if err != nil {
		return err
	}
	raw["messages"] = newMessages

	out, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	req.Body = out
	return nil
}

// CountTokens does a coarse character-based estimate (~4 chars per
// token). The real tokenizer lives outside the hot path; this is a
// best-effort pre-flight estimate.
func (p *Provider) CountTokens(req *provider.PassthroughRequest) (int, error) {
	msgs, err := p.ExtractMessages(req)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, m := range msgs {
		total += approxTokens(m.Content)
	}
	return total, nil
}

func approxTokens(s string) int {
	if s == "" {
		return 0
	}
	return (len(s) + 3) / 4
}

// IsStream returns true if the body sets stream:true. Used by the
// handler to pick Forward vs Stream.
func IsStream(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	var probe struct {
		Stream bool `json:"stream"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return false
	}
	return probe.Stream
}

// Forward issues the upstream call and buffers the response.
func (p *Provider) Forward(ctx context.Context, req *provider.PassthroughRequest, key *provider.ProviderKey) (*provider.PassthroughResponse, error) {
	httpReq, err := p.buildUpstream(ctx, req, key)
	if err != nil {
		return nil, err
	}
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", provider.ErrUpstream, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", provider.ErrUpstream, err)
	}
	return &provider.PassthroughResponse{
		Status:  resp.StatusCode,
		Headers: provider.CopyHeaders(resp.Header),
		Body:    body,
	}, nil
}

// Stream issues the upstream call and returns a reader over the SSE
// body. Caller MUST Close.
func (p *Provider) Stream(ctx context.Context, req *provider.PassthroughRequest, key *provider.ProviderKey) (provider.StreamReader, error) {
	httpReq, err := p.buildUpstream(ctx, req, key)
	if err != nil {
		return nil, err
	}
	// Make sure the upstream doesn't try to gzip the SSE.
	httpReq.Header.Set("Accept", "text/event-stream")
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", provider.ErrUpstream, err)
	}
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%w: status %d: %s", provider.ErrUpstream, resp.StatusCode, string(body))
	}
	return newSSEReader(resp.Body, resp.Header), nil
}

func (p *Provider) buildUpstream(ctx context.Context, req *provider.PassthroughRequest, key *provider.ProviderKey) (*http.Request, error) {
	if key == nil || key.Master == "" {
		return nil, fmt.Errorf("%w: missing master key", provider.ErrUpstream)
	}
	path := strings.TrimPrefix(req.Path, "/openai")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	url := p.baseURL + path
	if req.Query != "" {
		url += "?" + req.Query
	}

	var body io.Reader
	if len(req.Body) > 0 {
		body = bytes.NewReader(req.Body)
	}
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, url, body)
	if err != nil {
		return nil, err
	}
	// Forward client headers, then overwrite the auth headers with the
	// company's master key. We strip Host and any inbound auth.
	for k, vs := range req.Headers {
		if isHopByHop(k) || strings.EqualFold(k, "Authorization") || strings.EqualFold(k, "Host") {
			continue
		}
		for _, v := range vs {
			httpReq.Header.Add(k, v)
		}
	}
	httpReq.Header.Set("Authorization", "Bearer "+key.Master)
	if key.Extra.OpenAIOrgID != "" {
		httpReq.Header.Set("OpenAI-Organization", key.Extra.OpenAIOrgID)
	}
	if httpReq.Header.Get("Content-Type") == "" && len(req.Body) > 0 {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	return httpReq, nil
}

func needsBodyParse(req *provider.PassthroughRequest) bool {
	// Only POSTed JSON bodies with a "messages" or "input" field are
	// subject to message extraction. Models GET has no body; embeddings
	// has "input" which we treat as not chat.
	if req == nil {
		return false
	}
	switch {
	case strings.HasSuffix(req.Path, "/chat/completions"):
		return true
	case strings.HasSuffix(req.Path, "/responses"):
		return true
	default:
		return false
	}
}

func isHopByHop(h string) bool {
	switch strings.ToLower(h) {
	case "connection", "proxy-connection", "keep-alive", "transfer-encoding",
		"te", "trailer", "upgrade", "proxy-authorization", "proxy-authenticate":
		return true
	}
	return false
}
