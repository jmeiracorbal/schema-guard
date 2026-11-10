CREATE TABLE schema_guard_integration.events (
    id INTEGER,
    created_at DATE NOT NULL
) PARTITION BY RANGE (created_at);
