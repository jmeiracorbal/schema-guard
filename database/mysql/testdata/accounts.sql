CREATE TABLE sg_accounts (
 id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
 email VARCHAR(100) NOT NULL,
 state VARCHAR(15) NOT NULL DEFAULT 'active',
 normalized_email VARCHAR(100) GENERATED ALWAYS AS (LOWER(email)) STORED,
 UNIQUE KEY uq_account_email (email),
 INDEX idx_account_prefix (email(12)),
 CONSTRAINT chk_account_state CHECK (state <> '')
) ENGINE=InnoDB;
