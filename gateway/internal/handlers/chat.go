package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/Hesper-Labs/leakshield/gateway/internal/provider"
)

// maxRequestBody is the largest body the gateway will buffer for
// inspection. Above this we 413 the client. The number is generous
// enough to cover any reasonable chat / multimodal payload but keeps
// memory bounded.
//
// TODO(phase-policy): make this per-tenant via policy.config.
const maxRequestBody = 8 * 1024 * 1024 // 8 MiB

// VirtualKey is the placeholder identity carried through the request.
// The real type lives behind a DB lookup; for now we just remember the
// presented key prefix and tenant hint so log lines have something.
//
// TODO(phase-auth): replace with the resolved virtual_keys row +
// tenant identifier from internal/auth.
type VirtualKey struct {
	LookupPrefix string
	CompanyID    string
	Env          string
}

// stubVirtualKey reads "Authorization: Bearer gw_<env>_..." and returns
// a placeholder VirtualKey. Returns nil if the header is missing or
// malformed; the caller turns that into 401.
func stubVirtualKey(r *http.Request) *VirtualKey {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		// Anthropic / Azure callers may send the key on a different
		// header; sniff those too so the placeholder is forgiving.
		if v := r.Header.Get("x-api-key"); strings.HasPrefix(v, "gw_") {
			return parseStubBearer(v)
		}
		if v := r.Header.Get("api-key"); strings.HasPrefix(v, "gw_") {
			return parseStubBearer(v)
		}
		if v := r.URL.Query().Get("key"); strings.HasPrefix(v, "gw_") {
			return parseStubBearer(v)
		}
		return nil
	}
	return parseStubBearer(strings.TrimPrefix(auth, "Bearer "))
}

func parseStubBearer(token string) *VirtualKey {
	if !strings.HasPrefix(token, "gw_") {
		return nil
	}
	parts := strings.Split(token, "_")
	if len(parts) < 3 {
		return nil
	}
	return &VirtualKey{
		LookupPrefix: parts[0] + "_" + parts[1] + "_" + parts[2],
		Env:          parts[1],
		// CompanyID is unknown until we hit the DB.
		CompanyID: "stub-company",
	}
}

// ChatHandler returns an http.HandlerFunc that proxies a request through
// the named provider adapter. It implements the request flow described
// in docs/architecture.md:
//
//  1. Authenticate the virtual key (placeholder).
//  2. Resolve the master ProviderKey for the tenant + provider.
//  3. Extract messages for inspection.
//  4. Call the inspector (placeholder ALLOW).
//  5. Forward (or Stream) to the upstream provider.
//  6. Audit log (placeholder slog INFO).
//
// The adapter is constructed from the registry on every call. That keeps
// the handler itself stateless so wiring can be changed without
// touching the server bootstrap.
func ChatHandler(logger *slog.Logger, providerName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.With("provider", providerName, "path", r.URL.Path)

		// 1. Auth (placeholder).
		// TODO(phase-auth): replace with the real lookup pipeline:
		//   - Parse with auth.Parse.
		//   - Argon2id verify against virtual_keys.secret_hash.
		//   - SET LOCAL app.tenant_id for RLS.
		//   - Cache the verified row in the LRU.
		vkey := stubVirtualKey(r)
		if vkey == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error":   "unauthorized",
				"message": "missing or malformed virtual key",
			})
			return
		}

		// 2. Resolve master ProviderKey (placeholder: env vars).
		// TODO(phase-keys): replace with `provider_keys` row lookup +
		// envelope decryption with the tenant DEK.
		masterKey, err := devMasterKey(providerName)
		if err != nil {
			log.Error("master key unavailable", "error", err)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error":   "no_master_key",
				"message": fmt.Sprintf("no master key configured for provider %q", providerName),
			})
			return
		}

		// 3. Construct the adapter and parse the request.
		adapter, err := provider.Lookup(providerName)
		if err != nil {
			log.Error("provider lookup failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error":   "provider_unavailable",
				"message": err.Error(),
			})
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBody))
		if err != nil {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
				"error":   "body_too_large",
				"message": err.Error(),
			})
			return
		}
		_ = r.Body.Close()

		preq := &provider.PassthroughRequest{
			Method:  r.Method,
			Path:    r.URL.Path,
			Query:   r.URL.RawQuery,
			Headers: provider.CopyHeaders(r.Header),
			Body:    body,
		}

		// 4. Extract messages + inspect (placeholder ALLOW).
		messages, err := adapter.ExtractMessages(preq)
		if err != nil {
			log.Warn("message extraction failed", "error", err)
			// Extraction failures shouldn't block forwarding — the body
			// might just be a non-chat shape (embeddings, models). We
			// fall through with an empty slice; the inspector
			// placeholder always allows so this stays correct.
			messages = nil
		}

		decision, masked := inspectorPlaceholder(r.Context(), vkey, messages)
		if decision == DecisionBlock {
			writeJSON(w, http.StatusForbidden, map[string]string{
				"error":   "blocked_by_dlp",
				"message": "request blocked by DLP policy",
			})
			auditPlaceholder(log, vkey, providerName, "BLOCK")
			return
		}
		if decision == DecisionMask && masked != nil {
			if err := adapter.InjectMessages(preq, masked); err != nil {
				log.Error("message injection failed", "error", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{
					"error":   "mask_failed",
					"message": err.Error(),
				})
				return
			}
		}

		// 5. Decide stream vs non-stream and forward.
		if isStreamRequest(providerName, preq) {
			handleStream(r.Context(), w, log, adapter, preq, masterKey, vkey, providerName)
			return
		}
		handleForward(r.Context(), w, log, adapter, preq, masterKey, vkey, providerName)
	}
}

// Decision is the placeholder enum used until the gRPC inspector client
// lands. It mirrors leakshield.inspector.v1.Decision.
type Decision int

const (
	// DecisionAllow forwards the request as-is.
	DecisionAllow Decision = iota
	// DecisionBlock rejects the request with 403.
	DecisionBlock
	// DecisionMask rewrites the messages and forwards.
	DecisionMask
)

// inspectorPlaceholder is a stand-in for the real gRPC inspector. The
// production client will call leakshield.inspector.v1.InspectPrompt with
// the messages and the policy.config blob.
//
// TODO(phase-inspector): replace with the gRPC client. The signature
// here is intentionally close to the real one so the swap is mechanical.
func inspectorPlaceholder(_ context.Context, _ *VirtualKey, _ []provider.Message) (Decision, []provider.Message) {
	return DecisionAllow, nil
}

// auditPlaceholder writes a structured log line in lieu of the real
// audit_log INSERT. The hash-chained, tenant-partitioned table comes
// later.
//
// TODO(phase-audit): replace with the audit_log writer that emits to
// the bounded ring buffer + Postgres COPY worker.
func auditPlaceholder(log *slog.Logger, vkey *VirtualKey, providerName, decision string) {
	log.Info("audit",
		"company_id", vkey.CompanyID,
		"key_prefix", vkey.LookupPrefix,
		"provider", providerName,
		"decision", decision,
	)
}

func devMasterKey(providerName string) (*provider.ProviderKey, error) {
	// TODO(phase-keys): replace with envelope-decrypted tenant key. The
	// env-var path is dev-only convenience; production refuses to start
	// without the real KMS pipeline.
	switch providerName {
	case "openai":
		k := os.Getenv("LEAKSHIELD_DEV_OPENAI_KEY")
		if k == "" {
			return nil, errors.New("LEAKSHIELD_DEV_OPENAI_KEY unset")
		}
		return &provider.ProviderKey{
			Master: k,
			Extra:  provider.ProviderKeyExtra{OpenAIOrgID: os.Getenv("LEAKSHIELD_DEV_OPENAI_ORG")},
		}, nil
	case "anthropic":
		k := os.Getenv("LEAKSHIELD_DEV_ANTHROPIC_KEY")
		if k == "" {
			return nil, errors.New("LEAKSHIELD_DEV_ANTHROPIC_KEY unset")
		}
		return &provider.ProviderKey{Master: k}, nil
	case "google":
		k := os.Getenv("LEAKSHIELD_DEV_GOOGLE_KEY")
		if k == "" {
			return nil, errors.New("LEAKSHIELD_DEV_GOOGLE_KEY unset")
		}
		return &provider.ProviderKey{Master: k}, nil
	case "azure":
		k := os.Getenv("LEAKSHIELD_DEV_AZURE_KEY")
		if k == "" {
			return nil, errors.New("LEAKSHIELD_DEV_AZURE_KEY unset")
		}
		return &provider.ProviderKey{
			Master: k,
			Extra: provider.ProviderKeyExtra{
				AzureEndpoint:   os.Getenv("LEAKSHIELD_DEV_AZURE_ENDPOINT"),
				AzureAPIVersion: os.Getenv("LEAKSHIELD_DEV_AZURE_API_VERSION"),
			},
		}, nil
	}
	return nil, fmt.Errorf("unknown provider %q", providerName)
}

// isStreamRequest checks whether the client asked for SSE. OpenAI /
// Anthropic / Azure use a body field; Google encodes it in the URL.
func isStreamRequest(providerName string, req *provider.PassthroughRequest) bool {
	switch providerName {
	case "google":
		return strings.Contains(req.Path, ":streamGenerateContent")
	default:
		if len(req.Body) == 0 {
			return false
		}
		var probe struct {
			Stream bool `json:"stream"`
		}
		if err := json.Unmarshal(req.Body, &probe); err != nil {
			return false
		}
		return probe.Stream
	}
}

func handleForward(
	ctx context.Context,
	w http.ResponseWriter,
	log *slog.Logger,
	adapter provider.Provider,
	req *provider.PassthroughRequest,
	key *provider.ProviderKey,
	vkey *VirtualKey,
	providerName string,
) {
	resp, err := adapter.Forward(ctx, req, key)
	if err != nil {
		log.Error("upstream forward failed", "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error":   "upstream_error",
			"message": err.Error(),
		})
		return
	}
	for k, vs := range resp.Headers {
		// Drop any hop-by-hop / transport-control headers from the
		// upstream so they don't confuse the client. The shared SSE
		// reader already handles these for streams.
		if isHopByHopResponse(k) {
			continue
		}
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.Status)
	_, _ = w.Write(resp.Body)
	auditPlaceholder(log, vkey, providerName, "ALLOW")
}

func handleStream(
	ctx context.Context,
	w http.ResponseWriter,
	log *slog.Logger,
	adapter provider.Provider,
	req *provider.PassthroughRequest,
	key *provider.ProviderKey,
	vkey *VirtualKey,
	providerName string,
) {
	reader, err := adapter.Stream(ctx, req, key)
	if err != nil {
		log.Error("upstream stream failed", "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error":   "upstream_error",
			"message": err.Error(),
		})
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Error("response writer does not support flushing")
		return
	}

	for {
		chunk, err := reader.Next(ctx)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			log.Warn("stream interrupted", "error", err)
			return
		}
		if _, werr := w.Write(chunk.Raw); werr != nil {
			log.Warn("client disconnected mid-stream", "error", werr)
			return
		}
		flusher.Flush()
	}
	auditPlaceholder(log, vkey, providerName, "ALLOW_STREAM")
}

func isHopByHopResponse(h string) bool {
	switch strings.ToLower(h) {
	case "connection", "transfer-encoding", "keep-alive", "proxy-connection",
		"upgrade", "trailer", "te", "content-length":
		return true
	}
	return false
}
