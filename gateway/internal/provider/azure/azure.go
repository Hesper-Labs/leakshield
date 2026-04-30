// Package azure implements the Azure OpenAI pass-through adapter.
//
// Azure OpenAI exposes the OpenAI-compatible Chat Completions / Embeddings
// schema, but the URL embeds the company's Azure resource, the deployment
// name, and an api-version query parameter. The adapter rewrites the URL
// using the company's stored Azure config (resource endpoint + per-model
// deployment map) and swaps the auth header.
//
// We accept two URL shapes from the client:
//
//	1. /azure/openai/deployments/{deployment}/chat/completions?api-version=...
//	   The client already chose a deployment; we keep it as-is.
//
//	2. /azure/openai/deployments/-/chat/completions?api-version=... (model: "gpt-4o")
//	   The client sent "-" as a placeholder; we resolve the deployment
//	   from the body's "model" field via the deployment map. This lets
//	   OpenAI-style clients work without knowing the company's
//	   deployment names.
//
// Body shape mirrors OpenAI's chat/completions; we delegate extract /
// inject to the OpenAI parser by reusing the same JSON shape.
//
// Auth header: api-key (not Authorization). We strip whatever the client
// sent on either header before forwarding.
package azure

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

// defaultAPIVersion is used when neither the request nor the
// ProviderKey.Extra.AzureAPIVersion sets one.
const defaultAPIVersion = "2024-08-01-preview"

// Provider is the Azure OpenAI adapter.
type Provider struct {
	httpClient *http.Client
}

// New constructs an Azure adapter.
func New() *Provider {
	t := &http.Transport{
		MaxIdleConnsPerHost: 256,
		IdleConnTimeout:     90 * time.Second,
		ForceAttemptHTTP2:   true,
	}
	return &Provider{httpClient: &http.Client{Transport: t}}
}

func init() {
	provider.Register("azure", func() (provider.Provider, error) {
		return New(), nil
	})
}

// Name returns the canonical provider name.
func (p *Provider) Name() string { return "azure" }

// Routes lists the chi-style URL patterns this adapter serves.
func (p *Provider) Routes() []provider.Route {
	return []provider.Route{
		{Method: http.MethodPost, Pattern: "/azure/openai/deployments/*"},
	}
}

// SupportedModels returns the same identifiers OpenAI uses; the actual
// deployments are tenant-specific and live in the per-company config.
func (p *Provider) SupportedModels() []string {
	return []string{
		"gpt-4o",
		"gpt-4o-mini",
		"gpt-4-turbo",
		"gpt-4",
		"gpt-3.5-turbo",
		"text-embedding-3-small",
		"text-embedding-3-large",
		"text-embedding-ada-002",
	}
}

// chatMessage / chatRequest mirror the OpenAI schema; Azure speaks the
// same wire format.
type chatMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream,omitempty"`
}

// ExtractMessages parses the chat/completions body. Embeddings requests
// have no messages and return an empty slice.
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
			Content: contentToString(m.Content),
		})
	}
	return out, nil
}

func contentToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
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
		_ = json.Unmarshal(typeRaw, &ptype)
		if ptype != "text" {
			continue
		}
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

// InjectMessages writes masked messages back into req.Body, leaving every
// other field untouched.
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

// Stream issues the upstream call and returns a reader.
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
	endpoint := key.Extra.AzureEndpoint
	if endpoint == "" {
		return nil, fmt.Errorf("%w: missing AzureEndpoint in provider key", provider.ErrUpstream)
	}
	endpoint = strings.TrimSuffix(endpoint, "/")

	// /azure/openai/deployments/{deployment}/chat/completions
	// becomes
	// {endpoint}/openai/deployments/{deployment}/chat/completions
	tail := strings.TrimPrefix(req.Path, "/azure")
	if !strings.HasPrefix(tail, "/openai/") {
		return nil, fmt.Errorf("%w: unexpected azure path %q", provider.ErrUnsupportedRoute, req.Path)
	}

	// Resolve the deployment placeholder if present.
	tail, err := p.resolveDeployment(tail, req.Body, key)
	if err != nil {
		return nil, err
	}

	target := endpoint + tail
	q, err := url.ParseQuery(req.Query)
	if err != nil {
		q = url.Values{}
	}
	if q.Get("api-version") == "" {
		v := key.Extra.AzureAPIVersion
		if v == "" {
			v = defaultAPIVersion
		}
		q.Set("api-version", v)
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
		if isHopByHop(k) ||
			strings.EqualFold(k, "Authorization") ||
			strings.EqualFold(k, "api-key") ||
			strings.EqualFold(k, "Host") {
			continue
		}
		for _, v := range vs {
			httpReq.Header.Add(k, v)
		}
	}
	httpReq.Header.Set("api-key", key.Master)
	if httpReq.Header.Get("Content-Type") == "" && len(req.Body) > 0 {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	return httpReq, nil
}

// resolveDeployment substitutes a "-" placeholder in the URL path with
// the deployment name from the AzureDeployments map keyed by the body's
// "model" field.
func (p *Provider) resolveDeployment(tail string, body []byte, key *provider.ProviderKey) (string, error) {
	const placeholder = "/openai/deployments/-/"
	if !strings.HasPrefix(tail, placeholder) {
		return tail, nil
	}
	if len(body) == 0 {
		return "", fmt.Errorf("%w: deployment placeholder used but body is empty", provider.ErrInvalidBody)
	}
	var probe struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return "", fmt.Errorf("%w: %v", provider.ErrInvalidBody, err)
	}
	if probe.Model == "" {
		return "", fmt.Errorf("%w: deployment placeholder used but body has no model field", provider.ErrInvalidBody)
	}
	deployment := key.Extra.AzureDeployments[probe.Model]
	if deployment == "" {
		return "", fmt.Errorf("%w: no Azure deployment configured for model %q", provider.ErrUpstream, probe.Model)
	}
	return strings.Replace(tail, "/-/", "/"+deployment+"/", 1), nil
}

func needsBodyParse(req *provider.PassthroughRequest) bool {
	if req == nil {
		return false
	}
	return strings.Contains(req.Path, "/chat/completions")
}

func isHopByHop(h string) bool {
	switch strings.ToLower(h) {
	case "connection", "proxy-connection", "keep-alive", "transfer-encoding",
		"te", "trailer", "upgrade", "proxy-authorization", "proxy-authenticate":
		return true
	}
	return false
}
