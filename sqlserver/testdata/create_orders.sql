CREATE TABLE schema_guard_integration.orders (
    id INT NOT NULL CONSTRAINT pk_orders PRIMARY KEY,
    account_id INT NOT NULL,
    CONSTRAINT fk_orders_accounts FOREIGN KEY(account_id)
        REFERENCES schema_guard_integration.accounts(id) ON DELETE CASCADE
);
