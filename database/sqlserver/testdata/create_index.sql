CREATE NONCLUSTERED INDEX idx_accounts_state ON schema_guard_integration.accounts (state DESC) INCLUDE (email) WHERE state > 0;
