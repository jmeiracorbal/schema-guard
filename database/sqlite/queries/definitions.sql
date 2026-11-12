SELECT type, name, sql
FROM {{schema}}.sqlite_schema
WHERE type IN ('table', 'index')
  AND name NOT LIKE 'sqlite_%'
ORDER BY type, name;
