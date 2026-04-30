package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	// Side-effect imports register the adapters with the provider
	// registry so the handler can resolve them by name.
	"github.com/Hesper-Labs/leakshield/gateway/internal/provider/openai"
)

// TestChatHandler_OpenAIForward exercises the full handler path: virtual
// key auth (placeholder), provider lookup, message extraction (no-op
// inspector), upstream forward, response copy.
func TestChatHandler_OpenAIForward(t *testing.T) {
	var seenAuth string
	var seenBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		seenBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-1","object":"chat.completion"}`))
	}))
	defer upstream.Close()

	openai.SetTestBaseURL(upstream.URL)
	t.Cleanup(func() { openai.SetTestBaseURL("") })

	t.Setenv("LEAKSHIELD_DEV_OPENAI_KEY", "sk-real-master")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := ChatHandler(logger, "openai")

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"Hello!"}]}`
	r := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer gw_live_abcd1234_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
	if seenAuth != "Bearer sk-real-master" {
		t.Errorf("upstream saw auth %q, want master", seenAuth)
	}
	if string(seenBody) != body {
		t.Errorf("body altered:\nwant: %s\ngot:  %s", body, seenBody)
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got["id"] != "chatcmpl-1" {
		t.Errorf("response not copied through: %v", got)
	}
}

// TestChatHandler_OpenAIStream verifies the streaming path: SSE chunks
// are forwarded byte-identical to the client.
func TestChatHandler_OpenAIStream(t *testing.T) {
	stream := "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n" +
		"data: [DONE]\n\n"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(stream))
	}))
	defer upstream.Close()

	openai.SetTestBaseURL(upstream.URL)
	t.Cleanup(func() { openai.SetTestBaseURL("") })

	t.Setenv("LEAKSHIELD_DEV_OPENAI_KEY", "sk-real-master")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := ChatHandler(logger, "openai")

	body := `{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`
	r := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer gw_live_abcd1234_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", got)
	}
	if got := w.Header().Get("X-Accel-Buffering"); got != "no" {
		t.Errorf("X-Accel-Buffering = %q, want no", got)
	}
	if w.Body.String() != stream {
		t.Errorf("stream altered:\nwant: %q\ngot:  %q", stream, w.Body.String())
	}
}

func TestChatHandler_MissingAuthRejects(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := ChatHandler(logger, "openai")

	r := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-4o","messages":[]}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestChatHandler_MissingMasterKey(t *testing.T) {
	t.Setenv("LEAKSHIELD_DEV_OPENAI_KEY", "")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := ChatHandler(logger, "openai")

	r := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-4o","messages":[]}`))
	r.Header.Set("Authorization", "Bearer gw_live_abcd1234_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestStubVirtualKey_Variants(t *testing.T) {
	cases := []struct {
		name       string
		setHeader  func(*http.Request)
		setQuery   bool
		wantPrefix string
	}{
		{
			name:       "bearer",
			setHeader:  func(r *http.Request) { r.Header.Set("Authorization", "Bearer gw_live_abcd1234_secret") },
			wantPrefix: "gw_live_abcd1234",
		},
		{
			name:       "x-api-key",
			setHeader:  func(r *http.Request) { r.Header.Set("x-api-key", "gw_test_zzyy1122_secret") },
			wantPrefix: "gw_test_zzyy1122",
		},
		{
			name:       "api-key",
			setHeader:  func(r *http.Request) { r.Header.Set("api-key", "gw_live_abcd0000_secret") },
			wantPrefix: "gw_live_abcd0000",
		},
		{
			name:       "query key",
			setQuery:   true,
			wantPrefix: "gw_live_qqqq9999",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", nil)
			if c.setHeader != nil {
				c.setHeader(r)
			}
			if c.setQuery {
				q := r.URL.Query()
				q.Set("key", "gw_live_qqqq9999_secret")
				r.URL.RawQuery = q.Encode()
			}
			vk := stubVirtualKey(r)
			if vk == nil {
				t.Fatal("got nil virtual key")
			}
			if vk.LookupPrefix != c.wantPrefix {
				t.Errorf("got prefix %q, want %q", vk.LookupPrefix, c.wantPrefix)
			}
		})
	}
}
