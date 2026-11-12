CREATE TABLE accounts (
  id INTEGER PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  state INTEGER NOT NULL DEFAULT 0 CHECK (state >= 0),
  lower_email TEXT GENERATED ALWAYS AS (lower(email)) STORED,
  freeform
);
CREATE TABLE orders (
  id INTEGER PRIMARY KEY,
  account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  code TEXT NOT NULL,
  FOREIGN KEY (account_id) REFERENCES accounts(id)
);
CREATE INDEX idx_orders_lower_code ON orders(lower(code)) WHERE account_id IS NOT NULL;
CREATE TABLE settings (
  category TEXT NOT NULL,
  name TEXT NOT NULL,
  PRIMARY KEY(category,name)
) WITHOUT ROWID;
CREATE TABLE strict_events (
  id INTEGER PRIMARY KEY,
  payload TEXT NOT NULL
) STRICT;
