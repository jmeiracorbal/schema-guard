CREATE TABLE schema_guard_integration.accounts (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email TEXT COLLATE "C" NOT NULL,
    state VARCHAR(12) NOT NULL DEFAULT 'active',
    credit NUMERIC(12, 2) DEFAULT 0 CHECK (credit >= 0),
    normalized_email TEXT GENERATED ALWAYS AS (lower(email)) STORED,
    CONSTRAINT accounts_state_chk CHECK (state <> '')
);
