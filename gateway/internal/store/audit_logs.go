package store

import (
	"context"
	"encoding/json"
	"net/netip"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AuditLogEntry is a single proxied request as it lands in the audit log.
type AuditLogEntry struct {
	ID                  uuid.UUID
	RequestID           uuid.UUID
	CompanyID           uuid.UUID
	UserID              *uuid.UUID
	VirtualKeyID        *uuid.UUID
	Provider            string
	Model               string
	Endpoint            string
	Status              string // allowed | blocked | masked | error | partial_block
	BlockedCategories   []string
	VerdictReason       string
	InspectorID         string
	InspectorLatencyMs  int32
	PromptHash          []byte
	PromptPreview       string
	PromptEncrypted     []byte
	PromptNonce         []byte
	ResponseHash        []byte
	PromptTokens        int32
	CompletionTokens    int32
	CostUSDMicro        int64
	LatencyMs           int32
	ClientIP            *netip.Addr
	UserAgent           string
	PrevHash            []byte
	RowHash             []byte
	ConfigVersion       json.RawMessage // optional bookkeeping
}

// InsertAuditLog appends a single audit row.
func (d *DB) InsertAuditLog(ctx context.Context, e *AuditLogEntry) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.RequestID == uuid.Nil {
		e.RequestID = uuid.New()
	}
	return d.WithTenant(ctx, e.CompanyID.String(), func(tx pgx.Tx) error {
		var clientIPStr *string
		if e.ClientIP != nil {
			s := e.ClientIP.String()
			clientIPStr = &s
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO audit_logs (
				id, request_id, company_id, user_id, virtual_key_id,
				provider, model, endpoint, status,
				blocked_categories, verdict_reason, inspector_id, inspector_latency_ms,
				prompt_hash, prompt_preview, prompt_encrypted, prompt_nonce,
				response_hash, prompt_tokens, completion_tokens,
				cost_usd_micro, latency_ms, client_ip, user_agent,
				prev_hash, row_hash
			)
			VALUES (
				$1, $2, $3, $4, $5,
				$6, $7, $8, $9,
				$10, $11, $12, $13,
				$14, $15, $16, $17,
				$18, $19, $20,
				$21, $22, $23::inet, $24,
				$25, $26
			)
		`,
			e.ID, e.RequestID, e.CompanyID, e.UserID, e.VirtualKeyID,
			e.Provider, e.Model, e.Endpoint, e.Status,
			e.BlockedCategories, e.VerdictReason, e.InspectorID, e.InspectorLatencyMs,
			e.PromptHash, e.PromptPreview, e.PromptEncrypted, e.PromptNonce,
			e.ResponseHash, e.PromptTokens, e.CompletionTokens,
			e.CostUSDMicro, e.LatencyMs, clientIPStr, e.UserAgent,
			e.PrevHash, e.RowHash,
		)
		return err
	})
}
