CREATE TABLE schema_guard_integration.events_2026
PARTITION OF schema_guard_integration.events
FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');
