-- +goose Up
-- +goose StatementBegin

-- Provider model catalog with prices in USD micros per 1k tokens.
-- Source: each provider's official pricing page; refresh periodically.
-- 1 micro = 1/1_000_000 USD; e.g. 2_500 == $0.0025.

INSERT INTO provider_models (provider, model, input_usd_micro_per_1k, output_usd_micro_per_1k, context_window) VALUES
    -- OpenAI
    ('openai', 'gpt-4o',           2500,  10000, 128000),
    ('openai', 'gpt-4o-mini',       150,    600, 128000),
    ('openai', 'gpt-4-turbo',     10000,  30000, 128000),
    ('openai', 'gpt-3.5-turbo',     500,   1500,  16000),
    ('openai', 'text-embedding-3-small',  20,      0,   8191),
    ('openai', 'text-embedding-3-large', 130,      0,   8191),

    -- Anthropic
    ('anthropic', 'claude-opus-4-7',          15000, 75000, 200000),
    ('anthropic', 'claude-sonnet-4-6',         3000, 15000, 200000),
    ('anthropic', 'claude-haiku-4-5-20251001',  800,  4000, 200000),

    -- Google Gemini
    ('google', 'gemini-1.5-pro',     1250,  5000, 2000000),
    ('google', 'gemini-1.5-flash',     75,   300, 1000000),
    ('google', 'gemini-2.0-flash',    100,   400, 1000000),

    -- Azure OpenAI (mirrors OpenAI; per-deployment overrides happen elsewhere)
    ('azure', 'gpt-4o',           2500,  10000, 128000),
    ('azure', 'gpt-4o-mini',       150,    600, 128000),
    ('azure', 'gpt-35-turbo',      500,   1500,  16000)
ON CONFLICT (provider, model) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DELETE FROM provider_models;

-- +goose StatementEnd
