// Package anthropic implements the Anthropic Messages pass-through
// adapter.
//
// Anthropic's request shape differs from OpenAI's chat/completions in
// three ways the adapter has to handle:
//
//  1. The system prompt is a top-level field, not a "system" role inside
//     the messages array. It can be a plain string or an array of content
//     blocks (with cache_control etc.).
//  2. Each message's content can be a string OR an array of content
//     blocks (text, image, tool_use, tool_result, ...). We extract text
//     for inspection and leave non-text blocks intact in the wire body.
//  3. Auth is via the x-api-key header, not Authorization.
//
// Supported endpoints:
//
//	POST /anthropic/v1/messages
//	POST /anthropic/v1/messages/count_tokens
package anthropic

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

const upstreamBase = "https://api.anthropic.com"

// Provider is the Anthropic adapter.
type Provider struct {
	httpClient *http.Client
	baseURL    string
}

// New constructs an Anthropic adapter.
func New() *Provider {
	return &Provider{
		httpClient: newHTTPClient(),
		baseURL:    upstreamBase,
	}
}

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
	return &http.Client{Transport: t}
}

func init() {
	provider.Register("anthropic", func() (provider.Provider, error) {
		return New(), nil
	})
}

// Name returns the canonical provider name.
func (p *Provider) Name() string { return "anthropic" }

// Routes lists the chi-style URL patterns this adapter serves.
func (p *Provider) Routes() []provider.Route {
	return []provider.Route{
		{Method: http.MethodPost, Pattern: "/anthropic/v1/messages"},
		{Method: http.MethodPost, Pattern: "/anthropic/v1/messages/count_tokens"},
	}
}

// SupportedModels returns common Claude model identifiers.
func (p *Provider) SupportedModels() []string {
	return []string{
		"claude-opus-4",
		"claude-sonnet-4",
		"claude-haiku-4",
		"claude-3-5-sonnet-latest",
		"claude-3-5-haiku-latest",
		"claude-3-opus-latest",
	}
}

// messagesRequest is the subset of the Anthropic Messages schema we parse.
// We use json.RawMessage everywhere so the wire body roundtrips byte-for-
// byte except for the fields we explicitly rewrite.
type messagesRequest struct {
	Model    string             `json:"model"`
	System   json.RawMessage    `json:"system,omitempty"`
	Messages []messagesMessage  `json:"messages"`
	Stream   bool               `json:"stream,omitempty"`
}

type messagesMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// ExtractMessages returns the messages including the top-level system
// field as a synthetic "system" message at index 0 (only if a system
// prompt was set). Tool-use / tool-result blocks contribute their text
// representation; image blocks are skipped (they pass through but are
// not inspected as text).
func (p *Provider) ExtractMessages(req *provider.PassthroughRequest) ([]provider.Message, error) {
	if !needsBodyParse(req) {
		return nil, nil
	}
	if len(req.Body) == 0 {
		return nil, nil
	}
	var mr messagesRequest
	if err := json.Unmarshal(req.Body, &mr); err != nil {
		return nil, fmt.Errorf("%w: %v", provider.ErrInvalidBody, err)
	}

	out := make([]provider.Message, 0, len(mr.Messages)+1)
	if len(mr.System) > 0 {
		sys := anthropicContentToString(mr.System)
		if sys != "" {
			out = append(out, provider.Message{Role: "system", Content: sys})
		}
	}
	for _, m := range mr.Messages {
		out = append(out, provider.Message{
			Role:    m.Role,
			Content: anthropicContentToString(m.Content),
		})
	}
	return out, nil
}

// anthropicContentToString flattens a content field for inspection. The
// Anthropic schema allows either a plain string or an array of blocks;
// each block has a "type" and a type-specific shape. We extract:
//
//   - "text": block.text
//   - "tool_use":     "[tool_use:" + name + " input=" + json + "]"
//   - "tool_result":  block.content (string or text-block array)
//
// Images and other types are skipped entirely (they are still forwarded
// in the original body byte-for-byte).
func anthropicContentToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var blocks []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	var b strings.Builder
	appendStr := func(s string) {
		if s == "" {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(s)
	}
	for _, blk := range blocks {
		typeRaw, ok := blk["type"]
		if !ok {
			continue
		}
		var btype string
		if err := json.Unmarshal(typeRaw, &btype); err != nil {
			continue
		}
		switch btype {
		case "text":
			if textRaw, ok := blk["text"]; ok {
				var s string
				if err := json.Unmarshal(textRaw, &s); err == nil {
					appendStr(s)
				}
			}
		case "tool_use":
			var name string
			if nameRaw, ok := blk["name"]; ok {
				_ = json.Unmarshal(nameRaw, &name)
			}
			input := blk["input"]
			appendStr(fmt.Sprintf("[tool_use:%s input=%s]", name, string(input)))
		case "tool_result":
			if contentRaw, ok := blk["content"]; ok {
				appendStr(anthropicContentToString(contentRaw))
			}
		}
	}
	return b.String()
}

// InjectMessages writes masked content back into the system field and
// the messages array. We rewrite block contents in place so that
// cache_control, tool_use, and other metadata survive intact.
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

	idx := 0
	// Optional system prompt sits at index 0 if present in the original.
	if sysRaw, ok := raw["system"]; ok && len(sysRaw) > 0 && idx < len(masked) {
		newSys, err := injectAnthropicContent(sysRaw, masked[idx].Content)
		if err != nil {
			return err
		}
		raw["system"] = newSys
		idx++
	}

	rawMessages, ok := raw["messages"]
	if ok {
		var origs []map[string]json.RawMessage
		if err := json.Unmarshal(rawMessages, &origs); err != nil {
			return fmt.Errorf("%w: %v", provider.ErrInvalidBody, err)
		}
		if len(origs) != len(masked)-idx {
			return fmt.Errorf("%w: masked count %d != extractable %d",
				provider.ErrInvalidBody, len(masked), len(origs)+idx)
		}
		for i := range origs {
			contentRaw, hasContent := origs[i]["content"]
			if !hasContent {
				contentRaw = json.RawMessage(`""`)
			}
			newContent, err := injectAnthropicContent(contentRaw, masked[idx+i].Content)
			if err != nil {
				return err
			}
			origs[i]["content"] = newContent
		}
		newMessages, err := json.Marshal(origs)
		if err != nil {
			return err
		}
		raw["messages"] = newMessages
	}

	out, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	req.Body = out
	return nil
}

// injectAnthropicContent rewrites the textual portion of a content
// field. If the original was a plain string, the result is a plain
// string. If it was an array of blocks, every "text" block is
// concatenated and replaced with a single text block containing the
// masked content; non-text blocks (images, tool_use, tool_result) are
// preserved in their original positions.
func injectAnthropicContent(orig json.RawMessage, masked string) (json.RawMessage, error) {
	// Plain string.
	var s string
	if err := json.Unmarshal(orig, &s); err == nil {
		return json.Marshal(masked)
	}
	var blocks []map[string]json.RawMessage
	if err := json.Unmarshal(orig, &blocks); err != nil {
		// Unknown shape; fall back to a plain string.
		return json.Marshal(masked)
	}

	out := make([]map[string]json.RawMessage, 0, len(blocks))
	textInjected := false
	for _, blk := range blocks {
		typeRaw, ok := blk["type"]
		if !ok {
			out = append(out, blk)
			continue
		}
		var btype string
		_ = json.Unmarshal(typeRaw, &btype)
		if btype == "text" {
			if !textInjected {
				maskedJSON, err := json.Marshal(masked)
				if err != nil {
					return nil, err
				}
				blk["text"] = maskedJSON
				out = append(out, blk)
				textInjected = true
			}
			// Drop subsequent text blocks; their content has been merged.
			continue
		}
		out = append(out, blk)
	}
	if !textInjected {
		maskedJSON, err := json.Marshal(masked)
		if err != nil {
			return nil, err
		}
		typeJSON, _ := json.Marshal("text")
		out = append(out, map[string]json.RawMessage{
			"type": typeJSON,
			"text": maskedJSON,
		})
	}
	return json.Marshal(out)
}

// CountTokens estimates input tokens (~4 chars/token).
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

// IsStream returns true if the body sets stream:true.
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

// Stream issues the upstream call and returns a reader over the SSE body.
func (p *Provider) Stream(ctx context.Context, req *provider.PassthroughRequest, key *provider.ProviderKey) (provider.StreamReader, error) {
	httpReq, err := p.buildUpstream(ctx, req, key)
	if err != nil {
		return nil, err
	}
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
	return provider.NewSSEReader(resp.Body, resp.Header), nil
}

func (p *Provider) buildUpstream(ctx context.Context, req *provider.PassthroughRequest, key *provider.ProviderKey) (*http.Request, error) {
	if key == nil || key.Master == "" {
		return nil, fmt.Errorf("%w: missing master key", provider.ErrUpstream)
	}
	path := strings.TrimPrefix(req.Path, "/anthropic")
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
	for k, vs := range req.Headers {
		if isHopByHop(k) || strings.EqualFold(k, "x-api-key") || strings.EqualFold(k, "Authorization") || strings.EqualFold(k, "Host") {
			continue
		}
		for _, v := range vs {
			httpReq.Header.Add(k, v)
		}
	}
	httpReq.Header.Set("x-api-key", key.Master)
	if httpReq.Header.Get("anthropic-version") == "" {
		httpReq.Header.Set("anthropic-version", "2023-06-01")
	}
	if httpReq.Header.Get("Content-Type") == "" && len(req.Body) > 0 {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	return httpReq, nil
}

func needsBodyParse(req *provider.PassthroughRequest) bool {
	if req == nil {
		return false
	}
	return strings.HasSuffix(req.Path, "/messages") ||
		strings.HasSuffix(req.Path, "/messages/count_tokens")
}

func isHopByHop(h string) bool {
	switch strings.ToLower(h) {
	case "connection", "proxy-connection", "keep-alive", "transfer-encoding",
		"te", "trailer", "upgrade", "proxy-authorization", "proxy-authenticate":
		return true
	}
	return false
}
