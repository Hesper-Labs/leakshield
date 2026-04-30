package openai

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
            {"role": "user", "content": "Hello!"}
        ]
    }`)
	p := New()
	got, err := p.ExtractMessages(&provider.PassthroughRequest{
		Path: "/openai/v1/chat/completions",
		Body: body,
	})
	if err != nil {
		t.Fatalf("ExtractMessages: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d messages, want 2", len(got))
	}
	if got[0].Role != "system" || got[0].Content != "You are helpful." {
		t.Errorf("system message wrong: %+v", got[0])
	}
	if got[1].Role != "user" || got[1].Content != "Hello!" {
		t.Errorf("user message wrong: %+v", got[1])
	}
}

func TestExtractMessages_MultipartContent(t *testing.T) {
	body := []byte(`{
        "model": "gpt-4o",
        "messages": [
            {"role": "user", "content": [
                {"type": "text", "text": "What's in this image?"},
                {"type": "image_url", "image_url": {"url": "data:..."}}
            ]}
        ]
    }`)
	p := New()
	got, err := p.ExtractMessages(&provider.PassthroughRequest{
		Path: "/openai/v1/chat/completions",
		Body: body,
	})
	if err != nil {
		t.Fatalf("ExtractMessages: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d messages, want 1", len(got))
	}
	if got[0].Content != "What's in this image?" {
		t.Errorf("content extraction wrong: %q", got[0].Content)
	}
}

func TestInjectMessagesMASK_Roundtrip(t *testing.T) {
	body := []byte(`{
        "model": "gpt-4o",
        "temperature": 0.5,
        "messages": [
            {"role": "user", "content": "My SSN is 123-45-6789"}
        ],
        "tools": [{"type": "function", "function": {"name": "search"}}]
    }`)
	p := New()
	req := &provider.PassthroughRequest{
		Path: "/openai/v1/chat/completions",
		Body: body,
	}
	masked := []provider.Message{
		{Role: "user", Content: "My SSN is [REDACTED:PII.SSN]"},
	}
	if err := p.InjectMessages(req, masked); err != nil {
		t.Fatalf("InjectMessages: %v", err)
	}

	var out map[string]json.RawMessage
	if err := json.Unmarshal(req.Body, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Non-message fields are preserved.
	if string(out["temperature"]) != "0.5" {
		t.Errorf("temperature lost: %s", out["temperature"])
	}
	if !strings.Contains(string(out["tools"]), "search") {
		t.Errorf("tools lost: %s", out["tools"])
	}
	// Messages are rewritten.
	got, err := p.ExtractMessages(req)
	if err != nil {
		t.Fatalf("ExtractMessages after inject: %v", err)
	}
	if len(got) != 1 || got[0].Content != "My SSN is [REDACTED:PII.SSN]" {
		t.Fatalf("masked content wrong: %+v", got)
	}
}

func TestForwardSwapsAuthHeader(t *testing.T) {
	var seenAuth string
	var seenBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		seenBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"chatcmpl-1","object":"chat.completion"}`))
	}))
	defer upstream.Close()

	p := newWithBase(upstream.URL)
	req := &provider.PassthroughRequest{
		Method:  http.MethodPost,
		Path:    "/openai/v1/chat/completions",
		Headers: http.Header{"Authorization": {"Bearer gw_live_virtual"}, "Content-Type": {"application/json"}},
		Body:    []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`),
	}
	resp, err := p.Forward(context.Background(), req, &provider.ProviderKey{Master: "sk-real-master"})
	if err != nil {
		t.Fatalf("Forward: %v", err)
	}
	if resp.Status != http.StatusOK {
		t.Fatalf("status %d", resp.Status)
	}
	if seenAuth != "Bearer sk-real-master" {
		t.Errorf("upstream saw auth %q, want master key bearer", seenAuth)
	}
	if strings.Contains(seenAuth, "gw_live_") {
		t.Errorf("virtual key leaked upstream: %q", seenAuth)
	}
	if string(seenBody) != string(req.Body) {
		t.Errorf("body altered:\nwant: %s\ngot:  %s", req.Body, seenBody)
	}
}

func TestStreamForwardsBytesUnchanged(t *testing.T) {
	stream := "data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n" +
		"data: [DONE]\n\n"

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(stream))
	}))
	defer upstream.Close()

	p := newWithBase(upstream.URL)
	req := &provider.PassthroughRequest{
		Method: http.MethodPost,
		Path:   "/openai/v1/chat/completions",
		Body:   []byte(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`),
	}
	r, err := p.Stream(context.Background(), req, &provider.ProviderKey{Master: "sk-real"})
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

func TestIsStream(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"no stream", `{"messages":[]}`, false},
		{"stream true", `{"stream":true,"messages":[]}`, true},
		{"stream false", `{"stream":false}`, false},
		{"empty", ``, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsStream([]byte(c.body)); got != c.want {
				t.Errorf("IsStream(%q) = %v, want %v", c.body, got, c.want)
			}
		})
	}
}

func TestCountTokens_Approx(t *testing.T) {
	p := New()
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello world"}]}`)
	n, err := p.CountTokens(&provider.PassthroughRequest{
		Path: "/openai/v1/chat/completions",
		Body: body,
	})
	if err != nil {
		t.Fatalf("CountTokens: %v", err)
	}
	// "hello world" is 11 chars / 4 ≈ 3.
	if n < 1 || n > 5 {
		t.Errorf("unexpected token estimate: %d", n)
	}
}
