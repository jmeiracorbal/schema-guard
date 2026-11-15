CREATE TABLE schema_guard_integration.accounts (
    id INT IDENTITY(1,1) NOT NULL CONSTRAINT pk_accounts PRIMARY KEY,
    email NVARCHAR(255) NOT NULL,
    state INT NOT NULL DEFAULT 0,
    normalized_email AS LOWER(email) PERSISTED,
    CONSTRAINT uq_accounts_email UNIQUE (email),
    CONSTRAINT ck_accounts_state CHECK (state >= 0)
);
