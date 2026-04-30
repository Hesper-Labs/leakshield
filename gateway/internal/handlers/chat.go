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

	"github.com/google/uuid"

	gwauth "github.com/Hesper-Labs/leakshield/gateway/internal/auth"
	"github.com/Hesper-Labs/leakshield/gateway/internal/keys"
	"github.com/Hesper-Labs/leakshield/gateway/internal/provider"
)

// maxRequestBody is the largest body the gateway will buffer for
// inspection. Above this we 413 the client.
//
// TODO(phase-policy): make this per-tenant via policy.config.
const maxRequestBody = 8 * 1024 * 1024 // 8 MiB

// ChatDeps bundles the wiring the chat handler needs. Any nil field falls
// back to a development-friendly stub so the handler is still usable in
// unit tests and during early bring-up.
type ChatDeps struct {
	Logger   *slog.Logger
	Verifier *gwauth.VirtualKeyVerifier
	Resolver *keys.Resolver
}

// ChatHandler is the legacy constructor that wires only a logger.
// Auth and master-key resolution fall back to the dev-mode stubs (parsing
// the bearer token without verifying it; reading master keys from
// LEAKSHIELD_DEV_<PROVIDER>_KEY env vars). Tests use this form.
func ChatHandler(logger *slog.Logger, providerName string) http.HandlerFunc {
	return ChatHandlerWithDeps(ChatDeps{Logger: logger}, providerName)
}

// ChatHandlerWithDeps is the production constructor. When Verifier and
// Resolver are non-nil the handler authenticates against the database and
// envelope-decrypts master provider keys.
func ChatHandlerWithDeps(deps ChatDeps, providerName string) http.HandlerFunc {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.With("provider", providerName, "path", r.URL.Path)

		// 1. Authenticate the virtual key.
		vctx, vkey, err := authenticate(r, deps.Verifier)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error":   "unauthorized",
				"message": err.Error(),
			})
			return
		}

		// 2. Per-key provider allowlist enforcement (DB-backed only).
		if vctx != nil && len(vctx.AllowedProviders) > 0 && !contains(vctx.AllowedProviders, providerName) {
			writeJSON(w, http.StatusForbidden, map[string]string{
				"error":   "provider_not_allowed",
				"message": fmt.Sprintf("virtual key cannot use provider %q", providerName),
			})
			return
		}

		// 3. Resolve master ProviderKey.
		masterKey, err := resolveMasterKey(r.Context(), providerName, vctx, deps.Resolver)
		if err != nil {
			log.Error("master key unavailable", "error", err)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error":   "no_master_key",
				"message": fmt.Sprintf("no master key configured for provider %q: %s", providerName, err.Error()),
			})
			return
		}

		// 4. Construct the adapter and parse the request.
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

		// 5. Extract messages + inspect (placeholder ALLOW until the
		// gRPC inspector client lands).
		messages, err := adapter.ExtractMessages(preq)
		if err != nil {
			log.Warn("message extraction failed", "error", err)
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

		// 6. Forward (or stream) to the upstream provider.
		if isStreamRequest(providerName, preq) {
			handleStream(r.Context(), w, log, adapter, preq, masterKey, vkey, providerName)
			return
		}
		handleForward(r.Context(), w, log, adapter, preq, masterKey, vkey, providerName)
	}
}

// VirtualKey is the placeholder identity carried through the request when
// the verifier hasn't been wired yet (tests, early bring-up).
type VirtualKey struct {
	LookupPrefix string
	CompanyID    string
	UserID       string
	Env          string
}

// authenticate returns either a fully-resolved VirtualKeyContext (DB-backed)
// or a placeholder VirtualKey (when no verifier is wired). The handler uses
// the context for allowlist enforcement and master-key resolution; the
// placeholder is only used for log lines.
func authenticate(r *http.Request, v *gwauth.VirtualKeyVerifier) (*gwauth.VirtualKeyContext, *VirtualKey, error) {
	presented := extractPresentedKey(r)
	if presented == "" {
		return nil, nil, errors.New("missing virtual key")
	}
	if v != nil {
		ctx, err := v.Verify(r.Context(), presented)
		if err != nil {
			return nil, nil, errors.New("invalid virtual key")
		}
		return ctx, &VirtualKey{
			LookupPrefix: lookupPrefixFromKey(presented),
			CompanyID:    ctx.TenantID.String(),
			UserID:       ctx.UserID.String(),
			Env:          envFromKey(presented),
		}, nil
	}
	stub := parseStubBearer(presented)
	if stub == nil {
		return nil, nil, errors.New("malformed virtual key")
	}
	return nil, stub, nil
}

func resolveMasterKey(ctx context.Context, providerName string, vctx *gwauth.VirtualKeyContext, resolver *keys.Resolver) (*provider.ProviderKey, error) {
	if vctx != nil && resolver != nil {
		plain, mk, err := resolver.MasterKey(ctx, vctx.TenantID, providerName)
		if err != nil {
			return nil, err
		}
		extra, err := decodeProviderConfig(providerName, mk.Config)
		if err != nil {
			return nil, err
		}
		return &provider.ProviderKey{Master: plain, Extra: extra}, nil
	}
	return devMasterKey(providerName)
}

func decodeProviderConfig(providerName string, raw []byte) (provider.ProviderKeyExtra, error) {
	out := provider.ProviderKeyExtra{}
	if len(raw) == 0 {
		return out, nil
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return out, fmt.Errorf("decode provider config: %w", err)
	}
	if v, ok := cfg["openai_org_id"].(string); ok {
		out.OpenAIOrgID = v
	}
	if v, ok := cfg["azure_endpoint"].(string); ok {
		out.AzureEndpoint = v
	}
	if v, ok := cfg["azure_api_version"].(string); ok {
		out.AzureAPIVersion = v
	}
	if v, ok := cfg["azure_deployments"].(map[string]any); ok {
		out.AzureDeployments = make(map[string]string, len(v))
		for k, vv := range v {
			if s, ok := vv.(string); ok {
				out.AzureDeployments[k] = s
			}
		}
	}
	_ = providerName
	return out, nil
}

func extractPresentedKey(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	if v := r.Header.Get("x-api-key"); v != "" {
		return v
	}
	if v := r.Header.Get("api-key"); v != "" {
		return v
	}
	if v := r.URL.Query().Get("key"); v != "" {
		return v
	}
	return ""
}

// stubVirtualKey is the legacy helper used by chat_test.go. Production
// code paths go through `authenticate`; this wrapper keeps the existing
// extraction-and-parse coverage intact.
func stubVirtualKey(r *http.Request) *VirtualKey {
	return parseStubBearer(extractPresentedKey(r))
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
		CompanyID:    "stub-company",
	}
}

func lookupPrefixFromKey(token string) string {
	parts := strings.Split(token, "_")
	if len(parts) < 3 {
		return ""
	}
	return parts[0] + "_" + parts[1] + "_" + parts[2]
}

func envFromKey(token string) string {
	parts := strings.Split(token, "_")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
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
// TODO(phase-inspector): replace with the gRPC client.
func inspectorPlaceholder(_ context.Context, _ *VirtualKey, _ []provider.Message) (Decision, []provider.Message) {
	return DecisionAllow, nil
}

// auditPlaceholder writes a structured log line in lieu of the real
// audit_log INSERT.
//
// TODO(phase-audit): replace with the audit_log writer.
func auditPlaceholder(log *slog.Logger, vkey *VirtualKey, providerName, decision string) {
	if vkey == nil {
		log.Info("audit",
			"provider", providerName,
			"decision", decision,
		)
		return
	}
	log.Info("audit",
		"company_id", vkey.CompanyID,
		"key_prefix", vkey.LookupPrefix,
		"user_id", vkey.UserID,
		"provider", providerName,
		"decision", decision,
	)
}

func devMasterKey(providerName string) (*provider.ProviderKey, error) {
	switch providerName {
	case "openai":
		k := os.Getenv("LEAKSHIELD_DEV_OPENAI_KEY")
		if k == "" {
			return nil, errors.New("LEAKSHIELD_DEV_OPENAI_KEY unset (no DB-backed key configured)")
		}
		return &provider.ProviderKey{
			Master: k,
			Extra:  provider.ProviderKeyExtra{OpenAIOrgID: os.Getenv("LEAKSHIELD_DEV_OPENAI_ORG")},
		}, nil
	case "anthropic":
		k := os.Getenv("LEAKSHIELD_DEV_ANTHROPIC_KEY")
		if k == "" {
			return nil, errors.New("LEAKSHIELD_DEV_ANTHROPIC_KEY unset (no DB-backed key configured)")
		}
		return &provider.ProviderKey{Master: k}, nil
	case "google":
		k := os.Getenv("LEAKSHIELD_DEV_GOOGLE_KEY")
		if k == "" {
			return nil, errors.New("LEAKSHIELD_DEV_GOOGLE_KEY unset (no DB-backed key configured)")
		}
		return &provider.ProviderKey{Master: k}, nil
	case "azure":
		k := os.Getenv("LEAKSHIELD_DEV_AZURE_KEY")
		if k == "" {
			return nil, errors.New("LEAKSHIELD_DEV_AZURE_KEY unset (no DB-backed key configured)")
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

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

// keep uuid imported (used by some downstream callers when stubbing).
var _ uuid.UUID
