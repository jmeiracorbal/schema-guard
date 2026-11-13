CREATE UNIQUE INDEX accounts_normalized_email_idx
ON schema_guard_integration.accounts USING btree (lower(email)) INCLUDE (state)
WHERE email IS NOT NULL;
