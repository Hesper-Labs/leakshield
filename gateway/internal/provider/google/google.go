// Package google implements the Google Gemini (generativelanguage.googleapis.com)
// pass-through adapter for the v1beta API.
//
// The interesting parts:
//
//   - Authentication is via the "?key=..." query parameter OR an
//     "Authorization: Bearer ..." header. The adapter swaps whichever
//     the client used; if neither was provided we add the key as a
//     query param.
//   - The schema uses {role, parts:[{text:"..."}, {inline_data:...}]}
//     with the assistant role spelled "model". We translate "model" <->
//     "assistant" on extract / inject so the inspector sees the
//     normalized vocabulary.
//   - There is no top-level "system" field in v1beta the way Anthropic
//     has one; system instructions are sent as a separate
//     systemInstruction field. We extract it as a synthetic system
//     message at index 0.
//
// Supported endpoints:
//
//	POST /google/v1beta/models/{model}:generateContent
//	POST /google/v1beta/models/{model}:streamGenerateContent
//	POST /google/v1beta/models/{model}:countTokens
package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Hesper-Labs/leakshield/gateway/internal/provider"
)

const upstreamBase = "https://generativelanguage.googleapis.com"

// Provider is the Google Gemini adapter.
type Provider struct {
	httpClient *http.Client
	baseURL    string
}

// New constructs a Google adapter.
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
	provider.Register("google", func() (provider.Provider, error) {
		return New(), nil
	})
}

// Name returns the canonical provider name.
func (p *Provider) Name() string { return "google" }

// Routes lists the chi-style URL patterns this adapter serves.
func (p *Provider) Routes() []provider.Route {
	return []provider.Route{
		// chi treats `:` specially in patterns, so we use the wildcard
		// at the model segment and match the action suffix in the
		// adapter. The catch-all in server.go is "/models/*".
		{Method: http.MethodPost, Pattern: "/google/v1beta/models/*"},
	}
}

// SupportedModels returns common Gemini model identifiers.
func (p *Provider) SupportedModels() []string {
	return []string{
		"gemini-2.0-flash",
		"gemini-2.0-flash-thinking-exp",
		"gemini-1.5-pro",
		"gemini-1.5-flash",
		"gemini-1.5-flash-8b",
		"text-embedding-004",
	}
}

// generateRequest is the subset of the v1beta generateContent schema we
// parse for inspection. We use json.RawMessage so unknown fields and
// non-text parts roundtrip untouched.
type generateRequest struct {
	Contents          []generateContent  `json:"contents"`
	SystemInstruction *generateContent   `json:"systemInstruction,omitempty"`
}

type generateContent struct {
	Role  string                       `json:"role,omitempty"`
	Parts []map[string]json.RawMessage `json:"parts,omitempty"`
}

// ExtractMessages converts the Google contents/parts shape into
// normalized messages, with "model" -> "assistant" role mapping and
// systemInstruction surfaced as a synthetic system message at index 0.
func (p *Provider) ExtractMessages(req *provider.PassthroughRequest) ([]provider.Message, error) {
	if !needsBodyParse(req) {
		return nil, nil
	}
	if len(req.Body) == 0 {
		return nil, nil
	}
	var gr generateRequest
	if err := json.Unmarshal(req.Body, &gr); err != nil {
		return nil, fmt.Errorf("%w: %v", provider.ErrInvalidBody, err)
	}

	out := make([]provider.Message, 0, len(gr.Contents)+1)
	if gr.SystemInstruction != nil {
		s := googlePartsToString(gr.SystemInstruction.Parts)
		if s != "" {
			out = append(out, provider.Message{Role: "system", Content: s})
		}
	}
	for _, c := range gr.Contents {
		role := c.Role
		if role == "model" {
			role = "assistant"
		}
		if role == "" {
			role = "user"
		}
		out = append(out, provider.Message{
			Role:    role,
			Content: googlePartsToString(c.Parts),
		})
	}
	return out, nil
}

func googlePartsToString(parts []map[string]json.RawMessage) string {
	var b strings.Builder
	for _, part := range parts {
		if textRaw, ok := part["text"]; ok {
			var s string
			if err := json.Unmarshal(textRaw, &s); err == nil {
				if b.Len() > 0 {
					b.WriteString("\n")
				}
				b.WriteString(s)
			}
		}
	}
	return b.String()
}

// InjectMessages writes masked content back into systemInstruction and
// each contents[i].parts[*].text field in place. Non-text parts
// (inline_data, file_data, function_call, function_response) are
// preserved unchanged.
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
	if sysRaw, ok := raw["systemInstruction"]; ok && len(sysRaw) > 0 && idx < len(masked) {
		newSys, err := injectGoogleContent(sysRaw, masked[idx].Content)
		if err != nil {
			return err
		}
		raw["systemInstruction"] = newSys
		idx++
	}
	if contentsRaw, ok := raw["contents"]; ok {
		var origs []json.RawMessage
		if err := json.Unmarshal(contentsRaw, &origs); err != nil {
			return fmt.Errorf("%w: %v", provider.ErrInvalidBody, err)
		}
		if len(origs) != len(masked)-idx {
			return fmt.Errorf("%w: masked count %d != extractable %d",
				provider.ErrInvalidBody, len(masked), len(origs)+idx)
		}
		newContents := make([]json.RawMessage, len(origs))
		for i := range origs {
			n, err := injectGoogleContent(origs[i], masked[idx+i].Content)
			if err != nil {
				return err
			}
			newContents[i] = n
		}
		out, err := json.Marshal(newContents)
		if err != nil {
			return err
		}
		raw["contents"] = out
	}

	out, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	req.Body = out
	return nil
}

func injectGoogleContent(orig json.RawMessage, masked string) (json.RawMessage, error) {
	var content map[string]json.RawMessage
	if err := json.Unmarshal(orig, &content); err != nil {
		return nil, err
	}
	partsRaw, ok := content["parts"]
	if !ok {
		// No parts; create a single text part.
		mJSON, _ := json.Marshal(masked)
		newParts, _ := json.Marshal([]map[string]json.RawMessage{
			{"text": mJSON},
		})
		content["parts"] = newParts
		return json.Marshal(content)
	}
	var parts []map[string]json.RawMessage
	if err := json.Unmarshal(partsRaw, &parts); err != nil {
		return nil, err
	}
	out := make([]map[string]json.RawMessage, 0, len(parts))
	textInjected := false
	for _, part := range parts {
		if _, isText := part["text"]; isText {
			if !textInjected {
				mJSON, err := json.Marshal(masked)
				if err != nil {
					return nil, err
				}
				part["text"] = mJSON
				out = append(out, part)
				textInjected = true
			}
			continue
		}
		out = append(out, part)
	}
	if !textInjected {
		mJSON, err := json.Marshal(masked)
		if err != nil {
			return nil, err
		}
		out = append(out, map[string]json.RawMessage{"text": mJSON})
	}
	newParts, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	content["parts"] = newParts
	return json.Marshal(content)
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

// IsStream returns true if the path uses streamGenerateContent. (Google
// distinguishes streaming via the action verb in the URL, not a body
// flag.)
func IsStream(path string) bool {
	return strings.Contains(path, ":streamGenerateContent")
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

// Stream issues the upstream call and returns a reader. Google streams
// JSON arrays (one element per line) when alt=sse is requested; we
// expose them via the shared SSE reader, which works for line-delimited
// streams too because Google emits them with `\r\n` separators.
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
	path := strings.TrimPrefix(req.Path, "/google")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	target := p.baseURL + path

	// Decide auth style: keep whatever the client used.
	usedBearer := false
	for _, v := range req.Headers["Authorization"] {
		if strings.HasPrefix(strings.ToLower(v), "bearer ") {
			usedBearer = true
			break
		}
	}

	q, err := url.ParseQuery(req.Query)
	if err != nil {
		q = url.Values{}
	}
	if !usedBearer {
		// Default and most common: query-param auth. Replace whatever
		// virtual key was there with the company's master.
		q.Set("key", key.Master)
	} else {
		// Drop any client-provided key param so it doesn't clash with
		// the bearer token.
		q.Del("key")
	}
	if encoded := q.Encode(); encoded != "" {
		target += "?" + encoded
	}

	var body io.Reader
	if len(req.Body) > 0 {
		body = bytes.NewReader(req.Body)
	}
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, target, body)
	if err != nil {
		return nil, err
	}
	for k, vs := range req.Headers {
		if isHopByHop(k) || strings.EqualFold(k, "Authorization") || strings.EqualFold(k, "Host") {
			continue
		}
		for _, v := range vs {
			httpReq.Header.Add(k, v)
		}
	}
	if usedBearer {
		httpReq.Header.Set("Authorization", "Bearer "+key.Master)
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
	return strings.Contains(req.Path, ":generateContent") ||
		strings.Contains(req.Path, ":streamGenerateContent") ||
		strings.Contains(req.Path, ":countTokens")
}

func isHopByHop(h string) bool {
	switch strings.ToLower(h) {
	case "connection", "proxy-connection", "keep-alive", "transfer-encoding",
		"te", "trailer", "upgrade", "proxy-authorization", "proxy-authenticate":
		return true
	}
	return false
}
