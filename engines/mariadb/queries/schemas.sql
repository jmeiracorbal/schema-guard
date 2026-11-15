SELECT t.SCHEMA_NAME
FROM information_schema.SCHEMATA AS t
WHERE t.SCHEMA_NAME NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')
  AND (? IS NULL OR JSON_CONTAINS(?, JSON_QUOTE(t.SCHEMA_NAME)))
ORDER BY t.SCHEMA_NAME;
