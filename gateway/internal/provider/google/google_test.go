package google

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

func TestExtractMessages_GenerateContent(t *testing.T) {
	body := []byte(`{
        "systemInstruction": {"parts": [{"text": "You are a helpful assistant."}]},
        "contents": [
            {"role": "user", "parts": [{"text": "What's the weather?"}]},
            {"role": "model", "parts": [{"text": "I don't have live access."}]},
            {"role": "user", "parts": [{"text": "OK, just guess."}]}
        ]
    }`)
	p := New()
	got, err := p.ExtractMessages(&provider.PassthroughRequest{
		Path: "/google/v1beta/models/gemini-2.0-flash:generateContent",
		Body: body,
	})
	if err != nil {
		t.Fatalf("ExtractMessages: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("got %d messages, want 4: %+v", len(got), got)
	}
	if got[0].Role != "system" || got[0].Content != "You are a helpful assistant." {
		t.Errorf("system wrong: %+v", got[0])
	}
	if got[1].Role != "user" {
		t.Errorf("user 1 wrong: %+v", got[1])
	}
	if got[2].Role != "assistant" {
		t.Errorf("model -> assistant mapping failed: %+v", got[2])
	}
	if got[3].Role != "user" || got[3].Content != "OK, just guess." {
		t.Errorf("user 2 wrong: %+v", got[3])
	}
}

func TestInjectMessagesMASK_PreservesNonText(t *testing.T) {
	body := []byte(`{
        "contents": [
            {"role": "user", "parts": [
                {"text": "My SSN is 123-45-6789"},
                {"inline_data": {"mime_type": "image/png", "data": "iVBORw0KG..."}}
            ]}
        ],
        "generationConfig": {"temperature": 0.7}
    }`)
	p := New()
	req := &provider.PassthroughRequest{
		Path: "/google/v1beta/models/gemini-2.0-flash:generateContent",
		Body: body,
	}
	masked := []provider.Message{
		{Role: "user", Content: "My SSN is [REDACTED:PII.SSN]"},
	}
	if err := p.InjectMessages(req, masked); err != nil {
		t.Fatalf("InjectMessages: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(req.Body, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// generationConfig preserved.
	if !strings.Contains(string(raw["generationConfig"]), "temperature") {
		t.Errorf("generationConfig lost: %s", raw["generationConfig"])
	}
	// inline_data preserved.
	if !strings.Contains(string(raw["contents"]), "inline_data") {
		t.Errorf("inline_data lost: %s", raw["contents"])
	}
	got, err := p.ExtractMessages(req)
	if err != nil {
		t.Fatalf("ExtractMessages after inject: %v", err)
	}
	if len(got) != 1 || got[0].Content != "My SSN is [REDACTED:PII.SSN]" {
		t.Fatalf("masked content wrong: %+v", got)
	}
}

func TestForwardSwapsAuthQueryKey(t *testing.T) {
	var seenKey, seenAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenKey = r.URL.Query().Get("key")
		seenAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[]}`))
	}))
	defer upstream.Close()

	p := newWithBase(upstream.URL)
	req := &provider.PassthroughRequest{
		Method: http.MethodPost,
		Path:   "/google/v1beta/models/gemini-2.0-flash:generateContent",
		Query:  "key=AIzaSyVirtual",
		Headers: http.Header{
			"Content-Type": {"application/json"},
		},
		Body: []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`),
	}
	resp, err := p.Forward(context.Background(), req, &provider.ProviderKey{Master: "AIzaSyMaster"})
	if err != nil {
		t.Fatalf("Forward: %v", err)
	}
	if resp.Status != http.StatusOK {
		t.Fatalf("status %d", resp.Status)
	}
	if seenKey != "AIzaSyMaster" {
		t.Errorf("upstream saw key %q, want master", seenKey)
	}
	if strings.Contains(seenKey, "Virtual") {
		t.Errorf("virtual key leaked upstream: %q", seenKey)
	}
	if seenAuth != "" {
		t.Errorf("Authorization header should be empty in query-key path, got %q", seenAuth)
	}
}

func TestForwardSwapsAuthBearer(t *testing.T) {
	var seenAuth, seenKey string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		seenKey = r.URL.Query().Get("key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[]}`))
	}))
	defer upstream.Close()

	p := newWithBase(upstream.URL)
	req := &provider.PassthroughRequest{
		Method: http.MethodPost,
		Path:   "/google/v1beta/models/gemini-2.0-flash:generateContent",
		Headers: http.Header{
			"Authorization": {"Bearer ya29.virtual"},
			"Content-Type":  {"application/json"},
		},
		Body: []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`),
	}
	resp, err := p.Forward(context.Background(), req, &provider.ProviderKey{Master: "ya29.master"})
	if err != nil {
		t.Fatalf("Forward: %v", err)
	}
	if resp.Status != http.StatusOK {
		t.Fatalf("status %d", resp.Status)
	}
	if seenAuth != "Bearer ya29.master" {
		t.Errorf("upstream saw Authorization %q, want bearer master", seenAuth)
	}
	if seenKey != "" {
		t.Errorf("query key should be empty in bearer path, got %q", seenKey)
	}
}

func TestStreamForwardsBytesUnchanged(t *testing.T) {
	stream := "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Hello\"}]}}]}\n\n" +
		"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\" world\"}]}}]}\n\n"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(stream))
	}))
	defer upstream.Close()

	p := newWithBase(upstream.URL)
	req := &provider.PassthroughRequest{
		Method: http.MethodPost,
		Path:   "/google/v1beta/models/gemini-2.0-flash:streamGenerateContent",
		Body:   []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`),
	}
	r, err := p.Stream(context.Background(), req, &provider.ProviderKey{Master: "AIzaSyMaster"})
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
	if !IsStream("/google/v1beta/models/gemini-2.0-flash:streamGenerateContent") {
		t.Error("expected true for streamGenerateContent")
	}
	if IsStream("/google/v1beta/models/gemini-2.0-flash:generateContent") {
		t.Error("expected false for generateContent")
	}
}
