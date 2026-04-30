-- +goose Up
-- +goose StatementBegin

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Tenants: each company that uses the gateway has one row here.
CREATE TABLE companies (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         TEXT NOT NULL,
    slug         TEXT UNIQUE NOT NULL,
    dek_wrapped  BYTEA NOT NULL,         -- per-company DEK wrapped by the KEK
    kek_id       TEXT NOT NULL,          -- which KEK wrapped the DEK (rotation)
    settings     JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Employees: end users who consume LLM APIs through the gateway.
CREATE TABLE users (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id   UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    surname      TEXT,
    email        TEXT NOT NULL,
    phone        TEXT,
    department   TEXT,
    role         TEXT NOT NULL DEFAULT 'member',
    is_active    BOOLEAN NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (company_id, email)
);
CREATE INDEX users_company_active ON users (company_id) WHERE is_active;

-- Admin accounts: people who log in to the panel.
CREATE TABLE admins (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id      UUID REFERENCES companies(id) ON DELETE CASCADE,
    email           TEXT NOT NULL,
    password_hash   TEXT NOT NULL,
    role            TEXT NOT NULL,
    mfa_secret      TEXT,
    last_login_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (email),
    CHECK (role IN ('super_admin', 'company_admin', 'operator', 'auditor'))
);
CREATE INDEX admins_company ON admins (company_id) WHERE company_id IS NOT NULL;

-- Master provider keys: the company's keys with OpenAI / Anthropic / etc.
-- Stored encrypted under the company DEK (AES-256-GCM).
CREATE TABLE master_provider_keys (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id        UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    provider          TEXT NOT NULL,
    label             TEXT NOT NULL,
    api_key_cipher    BYTEA NOT NULL,
    api_key_nonce     BYTEA NOT NULL,
    config            JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_active         BOOLEAN NOT NULL DEFAULT true,
    last_tested_at    TIMESTAMPTZ,
    last_test_status  TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    rotated_from_id   UUID REFERENCES master_provider_keys(id) ON DELETE SET NULL,
    CHECK (provider IN ('openai', 'anthropic', 'google', 'azure'))
);
CREATE INDEX mpk_company_provider ON master_provider_keys (company_id, provider) WHERE is_active;

-- Virtual keys: gateway-issued keys assigned to individual employees.
CREATE TABLE virtual_keys (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id               UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    user_id                  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name                     TEXT NOT NULL,
    key_prefix               VARCHAR(32) UNIQUE NOT NULL,
    key_hash                 BYTEA NOT NULL,
    allowed_providers        TEXT[] NOT NULL DEFAULT '{}',
    allowed_models           TEXT[] NOT NULL DEFAULT '{}',
    rpm_limit                INT,
    tpm_limit                INT,
    monthly_token_limit      BIGINT,
    monthly_usd_micro_limit  BIGINT,
    expires_at               TIMESTAMPTZ,
    revoked_at               TIMESTAMPTZ,
    last_used_at             TIMESTAMPTZ,
    is_active                BOOLEAN GENERATED ALWAYS AS (revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now())) STORED,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX virtual_keys_company_user ON virtual_keys (company_id, user_id);
CREATE INDEX virtual_keys_active_lookup ON virtual_keys (key_prefix) WHERE revoked_at IS NULL;

-- DLP policies: per-company strategy choice + configuration.
CREATE TABLE dlp_policies (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id           UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    name                 TEXT NOT NULL,
    strategy             TEXT NOT NULL,
    config               JSONB NOT NULL,
    fail_mode            TEXT NOT NULL DEFAULT 'closed',
    response_inspection  BOOLEAN NOT NULL DEFAULT false,
    audit_full_prompt    BOOLEAN NOT NULL DEFAULT false,
    is_default           BOOLEAN NOT NULL DEFAULT false,
    version              INT NOT NULL DEFAULT 1,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (strategy IN ('mock', 'hybrid', 'specialized', 'judge', 'off')),
    CHECK (fail_mode IN ('closed', 'open', 'open_for_low_risk'))
);
CREATE UNIQUE INDEX dlp_policies_company_default ON dlp_policies (company_id) WHERE is_default;

-- Append-only policy version history (rollback + audit).
CREATE TABLE policy_versions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    policy_id       UUID NOT NULL REFERENCES dlp_policies(id) ON DELETE CASCADE,
    version         INT NOT NULL,
    content         JSONB NOT NULL,
    edited_by       UUID REFERENCES admins(id) ON DELETE SET NULL,
    test_results    JSONB,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (policy_id, version)
);

-- Audit log (monthly partitioned, hash-chained for tamper evidence).
CREATE TABLE audit_logs (
    id                        UUID NOT NULL,
    request_id                UUID NOT NULL,
    company_id                UUID NOT NULL,
    user_id                   UUID,
    virtual_key_id            UUID,
    provider                  TEXT NOT NULL,
    model                     TEXT,
    endpoint                  TEXT NOT NULL,
    status                    TEXT NOT NULL,
    blocked_categories        TEXT[],
    verdict_reason            TEXT,
    inspector_id              TEXT,
    inspector_latency_ms      INT,
    prompt_hash               BYTEA NOT NULL,
    prompt_preview            TEXT,
    prompt_encrypted          BYTEA,
    prompt_nonce              BYTEA,
    response_hash             BYTEA,
    prompt_tokens             INT,
    completion_tokens         INT,
    cost_usd_micro            BIGINT,
    latency_ms                INT,
    client_ip                 INET,
    user_agent                TEXT,
    prev_hash                 BYTEA,
    row_hash                  BYTEA,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id, created_at),
    CHECK (status IN ('allowed', 'blocked', 'masked', 'error', 'partial_block'))
) PARTITION BY RANGE (created_at);

-- Default partition until the worker creates monthly ones.
CREATE TABLE audit_logs_default PARTITION OF audit_logs DEFAULT;

CREATE INDEX audit_logs_company_created ON audit_logs (company_id, created_at DESC);
CREATE INDEX audit_logs_user_created ON audit_logs (company_id, user_id, created_at DESC) WHERE user_id IS NOT NULL;
CREATE INDEX audit_logs_blocked ON audit_logs (company_id, created_at DESC) WHERE status IN ('blocked', 'masked', 'partial_block');
CREATE INDEX audit_logs_request ON audit_logs (request_id);

-- Daily rollups for fast analytics charts.
CREATE TABLE usage_aggregates (
    company_id         UUID NOT NULL,
    user_id            UUID,
    virtual_key_id     UUID,
    provider           TEXT NOT NULL,
    model              TEXT,
    bucket             DATE NOT NULL,
    requests           BIGINT NOT NULL DEFAULT 0,
    blocked            BIGINT NOT NULL DEFAULT 0,
    masked             BIGINT NOT NULL DEFAULT 0,
    errored            BIGINT NOT NULL DEFAULT 0,
    prompt_tokens      BIGINT NOT NULL DEFAULT 0,
    completion_tokens  BIGINT NOT NULL DEFAULT 0,
    cost_usd_micro     BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (company_id, bucket, COALESCE(user_id, '00000000-0000-0000-0000-000000000000'::uuid),
                 COALESCE(virtual_key_id, '00000000-0000-0000-0000-000000000000'::uuid), provider, COALESCE(model, ''))
);
CREATE INDEX usage_aggregates_company_bucket ON usage_aggregates (company_id, bucket DESC);

-- Provider model catalog used for cost estimation.
CREATE TABLE provider_models (
    provider                  TEXT NOT NULL,
    model                     TEXT NOT NULL,
    input_usd_micro_per_1k    BIGINT NOT NULL,
    output_usd_micro_per_1k   BIGINT NOT NULL,
    context_window            INT,
    deprecated                BOOLEAN NOT NULL DEFAULT false,
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, model)
);

-- Row-level security for tenant isolation.
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE master_provider_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE virtual_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE dlp_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE policy_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE usage_aggregates ENABLE ROW LEVEL SECURITY;

-- Each transaction must SET LOCAL app.tenant_id before querying.
CREATE POLICY tenant_isolation ON users
    USING (company_id = current_setting('app.tenant_id', true)::uuid);
CREATE POLICY tenant_isolation ON master_provider_keys
    USING (company_id = current_setting('app.tenant_id', true)::uuid);
CREATE POLICY tenant_isolation ON virtual_keys
    USING (company_id = current_setting('app.tenant_id', true)::uuid);
CREATE POLICY tenant_isolation ON dlp_policies
    USING (company_id = current_setting('app.tenant_id', true)::uuid);
CREATE POLICY tenant_isolation ON policy_versions
    USING (policy_id IN (SELECT id FROM dlp_policies WHERE company_id = current_setting('app.tenant_id', true)::uuid));
CREATE POLICY tenant_isolation ON audit_logs
    USING (company_id = current_setting('app.tenant_id', true)::uuid);
CREATE POLICY tenant_isolation ON usage_aggregates
    USING (company_id = current_setting('app.tenant_id', true)::uuid);

-- Service role used by the gateway. RLS is NOT bypassed; the application
-- must always set app.tenant_id on every transaction.
CREATE ROLE leakshield_service NOINHERIT LOGIN PASSWORD 'set-via-secret';
GRANT USAGE ON SCHEMA public TO leakshield_service;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO leakshield_service;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO leakshield_service;
ALTER ROLE leakshield_service SET search_path = public;

-- updated_at trigger.
CREATE OR REPLACE FUNCTION trg_set_updated_at() RETURNS trigger AS $$
BEGIN NEW.updated_at = now(); RETURN NEW; END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER companies_updated BEFORE UPDATE ON companies FOR EACH ROW EXECUTE FUNCTION trg_set_updated_at();
CREATE TRIGGER users_updated BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION trg_set_updated_at();
CREATE TRIGGER dlp_policies_updated BEFORE UPDATE ON dlp_policies FOR EACH ROW EXECUTE FUNCTION trg_set_updated_at();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS usage_aggregates CASCADE;
DROP TABLE IF EXISTS audit_logs CASCADE;
DROP TABLE IF EXISTS policy_versions CASCADE;
DROP TABLE IF EXISTS dlp_policies CASCADE;
DROP TABLE IF EXISTS virtual_keys CASCADE;
DROP TABLE IF EXISTS master_provider_keys CASCADE;
DROP TABLE IF EXISTS provider_models CASCADE;
DROP TABLE IF EXISTS admins CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS companies CASCADE;

DROP FUNCTION IF EXISTS trg_set_updated_at();
DROP ROLE IF EXISTS leakshield_service;

-- +goose StatementEnd
