package anthropic

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

func TestExtractMessages_SystemAndCacheControl(t *testing.T) {
	body := []byte(`{
        "model": "claude-3-5-sonnet-latest",
        "system": [
            {"type": "text", "text": "You are a helpful assistant.", "cache_control": {"type": "ephemeral"}}
        ],
        "messages": [
            {"role": "user", "content": "Hello!"},
            {"role": "assistant", "content": [
                {"type": "text", "text": "Hi there."},
                {"type": "tool_use", "id": "tu1", "name": "lookup", "input": {"q": "weather"}}
            ]}
        ],
        "max_tokens": 1024
    }`)
	p := New()
	got, err := p.ExtractMessages(&provider.PassthroughRequest{
		Path: "/anthropic/v1/messages",
		Body: body,
	})
	if err != nil {
		t.Fatalf("ExtractMessages: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d messages, want 3 (system + 2): %+v", len(got), got)
	}
	if got[0].Role != "system" || got[0].Content != "You are a helpful assistant." {
		t.Errorf("system wrong: %+v", got[0])
	}
	if got[1].Role != "user" || got[1].Content != "Hello!" {
		t.Errorf("user wrong: %+v", got[1])
	}
	if got[2].Role != "assistant" || !strings.Contains(got[2].Content, "Hi there.") {
		t.Errorf("assistant wrong: %+v", got[2])
	}
	if !strings.Contains(got[2].Content, "tool_use:lookup") {
		t.Errorf("tool_use not surfaced: %+v", got[2])
	}
}

func TestExtractMessages_PlainStringSystem(t *testing.T) {
	body := []byte(`{
        "model": "claude-3-5-sonnet-latest",
        "system": "Be concise.",
        "messages": [
            {"role": "user", "content": "hi"}
        ]
    }`)
	p := New()
	got, err := p.ExtractMessages(&provider.PassthroughRequest{
		Path: "/anthropic/v1/messages",
		Body: body,
	})
	if err != nil {
		t.Fatalf("ExtractMessages: %v", err)
	}
	if len(got) != 2 || got[0].Content != "Be concise." {
		t.Fatalf("plain-string system not extracted: %+v", got)
	}
}

func TestInjectMessagesMASK_PreservesCacheControl(t *testing.T) {
	body := []byte(`{
        "model": "claude-3-5-sonnet-latest",
        "system": [
            {"type": "text", "text": "Be helpful.", "cache_control": {"type": "ephemeral"}}
        ],
        "messages": [
            {"role": "user", "content": "My SSN is 123-45-6789"}
        ],
        "max_tokens": 1024
    }`)
	p := New()
	req := &provider.PassthroughRequest{
		Path: "/anthropic/v1/messages",
		Body: body,
	}
	masked := []provider.Message{
		{Role: "system", Content: "Be helpful."},
		{Role: "user", Content: "My SSN is [REDACTED:PII.SSN]"},
	}
	if err := p.InjectMessages(req, masked); err != nil {
		t.Fatalf("InjectMessages: %v", err)
	}
	// max_tokens is preserved.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(req.Body, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(raw["max_tokens"]) != "1024" {
		t.Errorf("max_tokens lost: %s", raw["max_tokens"])
	}
	// cache_control survives.
	if !strings.Contains(string(raw["system"]), "cache_control") {
		t.Errorf("cache_control lost: %s", raw["system"])
	}
	// Re-extract to confirm.
	got, err := p.ExtractMessages(req)
	if err != nil {
		t.Fatalf("ExtractMessages after inject: %v", err)
	}
	if len(got) != 2 || got[1].Content != "My SSN is [REDACTED:PII.SSN]" {
		t.Fatalf("masked content wrong: %+v", got)
	}
}

func TestForwardSwapsAuthHeader(t *testing.T) {
	var seenKey, seenAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenKey = r.Header.Get("x-api-key")
		seenAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_01","type":"message"}`))
	}))
	defer upstream.Close()

	p := newWithBase(upstream.URL)
	req := &provider.PassthroughRequest{
		Method: http.MethodPost,
		Path:   "/anthropic/v1/messages",
		Headers: http.Header{
			"x-api-key":     {"sk-ant-virtual"},
			"Authorization": {"Bearer gw_live_virtual"},
			"Content-Type":  {"application/json"},
		},
		Body: []byte(`{"model":"claude-3-5-sonnet-latest","messages":[{"role":"user","content":"hi"}]}`),
	}
	resp, err := p.Forward(context.Background(), req, &provider.ProviderKey{Master: "sk-ant-real-master"})
	if err != nil {
		t.Fatalf("Forward: %v", err)
	}
	if resp.Status != http.StatusOK {
		t.Fatalf("status %d", resp.Status)
	}
	if seenKey != "sk-ant-real-master" {
		t.Errorf("upstream saw x-api-key %q, want master", seenKey)
	}
	if seenAuth != "" {
		t.Errorf("Authorization should be stripped, got %q", seenAuth)
	}
}

func TestStreamForwardsBytesUnchanged(t *testing.T) {
	stream := "event: message_start\ndata: {\"type\":\"message_start\"}\n\n" +
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(stream))
	}))
	defer upstream.Close()

	p := newWithBase(upstream.URL)
	req := &provider.PassthroughRequest{
		Method: http.MethodPost,
		Path:   "/anthropic/v1/messages",
		Body:   []byte(`{"model":"claude-3-5-sonnet-latest","stream":true,"messages":[{"role":"user","content":"hi"}]}`),
	}
	r, err := p.Stream(context.Background(), req, &provider.ProviderKey{Master: "sk-ant-real"})
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
