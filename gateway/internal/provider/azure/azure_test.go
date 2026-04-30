package azure

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Hesper-Labs/leakshield/gateway/internal/provider"
)

func TestExtractMessages_Chat(t *testing.T) {
	body := []byte(`{
        "model": "gpt-4o",
        "messages": [
            {"role": "system", "content": "You are helpful."},
            {"role": "user", "content": "Hi!"}
        ]
    }`)
	p := New()
	got, err := p.ExtractMessages(&provider.PassthroughRequest{
		Path: "/azure/openai/deployments/myprod/chat/completions",
		Body: body,
	})
	if err != nil {
		t.Fatalf("ExtractMessages: %v", err)
	}
	if len(got) != 2 || got[0].Role != "system" || got[1].Content != "Hi!" {
		t.Fatalf("messages wrong: %+v", got)
	}
}

func TestInjectMessagesMASK_Roundtrip(t *testing.T) {
	body := []byte(`{
        "model": "gpt-4o",
        "messages": [
            {"role": "user", "content": "My credit card is 4111 1111 1111 1111"}
        ],
        "temperature": 0.3
    }`)
	p := New()
	req := &provider.PassthroughRequest{
		Path: "/azure/openai/deployments/myprod/chat/completions",
		Body: body,
	}
	masked := []provider.Message{
		{Role: "user", Content: "My credit card is [REDACTED:PII.CREDIT_CARD]"},
	}
	if err := p.InjectMessages(req, masked); err != nil {
		t.Fatalf("InjectMessages: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(req.Body, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(raw["temperature"]) != "0.3" {
		t.Errorf("temperature lost: %s", raw["temperature"])
	}
	got, err := p.ExtractMessages(req)
	if err != nil {
		t.Fatalf("ExtractMessages after inject: %v", err)
	}
	if got[0].Content != "My credit card is [REDACTED:PII.CREDIT_CARD]" {
		t.Errorf("masked content wrong: %+v", got)
	}
}

func TestForwardSwapsAuthAndRewritesURL(t *testing.T) {
	var seenPath, seenAuth, seenAPIKey, seenAPIVersion string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenAPIVersion = r.URL.Query().Get("api-version")
		seenAuth = r.Header.Get("Authorization")
		seenAPIKey = r.Header.Get("api-key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-az-1","object":"chat.completion"}`))
	}))
	defer upstream.Close()

	p := New()
	req := &provider.PassthroughRequest{
		Method: http.MethodPost,
		Path:   "/azure/openai/deployments/myprod/chat/completions",
		Query:  "api-version=2024-08-01-preview",
		Headers: http.Header{
			"api-key":      {"virtual-azure"},
			"Content-Type": {"application/json"},
		},
		Body: []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`),
	}
	key := &provider.ProviderKey{
		Master: "real-azure-master",
		Extra: provider.ProviderKeyExtra{
			AzureEndpoint:   upstream.URL,
			AzureAPIVersion: "2024-08-01-preview",
		},
	}
	resp, err := p.Forward(context.Background(), req, key)
	if err != nil {
		t.Fatalf("Forward: %v", err)
	}
	if resp.Status != http.StatusOK {
		t.Fatalf("status %d", resp.Status)
	}
	if seenPath != "/openai/deployments/myprod/chat/completions" {
		t.Errorf("upstream path wrong: %q", seenPath)
	}
	if seenAPIKey != "real-azure-master" {
		t.Errorf("api-key wrong: %q", seenAPIKey)
	}
	if seenAuth != "" {
		t.Errorf("Authorization should be stripped, got %q", seenAuth)
	}
	if seenAPIVersion != "2024-08-01-preview" {
		t.Errorf("api-version wrong: %q", seenAPIVersion)
	}
}

func TestForwardResolvesDeploymentPlaceholder(t *testing.T) {
	var seenPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-1"}`))
	}))
	defer upstream.Close()

	p := New()
	req := &provider.PassthroughRequest{
		Method:  http.MethodPost,
		Path:    "/azure/openai/deployments/-/chat/completions",
		Headers: http.Header{"Content-Type": {"application/json"}},
		Body:    []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`),
	}
	key := &provider.ProviderKey{
		Master: "real",
		Extra: provider.ProviderKeyExtra{
			AzureEndpoint: upstream.URL,
			AzureDeployments: map[string]string{
				"gpt-4o":     "myprod-4o",
				"gpt-4-mini": "myprod-mini",
			},
		},
	}
	if _, err := p.Forward(context.Background(), req, key); err != nil {
		t.Fatalf("Forward: %v", err)
	}
	if seenPath != "/openai/deployments/myprod-4o/chat/completions" {
		t.Errorf("deployment not resolved: %q", seenPath)
	}
}

func TestStreamForwardsBytesUnchanged(t *testing.T) {
	stream := "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n" +
		"data: [DONE]\n\n"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(stream))
	}))
	defer upstream.Close()

	p := New()
	req := &provider.PassthroughRequest{
		Method:  http.MethodPost,
		Path:    "/azure/openai/deployments/myprod/chat/completions",
		Headers: http.Header{"Content-Type": {"application/json"}},
		Body:    []byte(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`),
	}
	key := &provider.ProviderKey{
		Master: "real",
		Extra:  provider.ProviderKeyExtra{AzureEndpoint: upstream.URL},
	}
	r, err := p.Stream(context.Background(), req, key)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	defer r.Close()
	var got strings.Builder
	for {
		chunk, err := r.Next(context.Background())
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		got.Write(chunk.Raw)
	}
	if got.String() != stream {
		t.Errorf("stream altered:\nwant: %q\ngot:  %q", stream, got.String())
	}
}
