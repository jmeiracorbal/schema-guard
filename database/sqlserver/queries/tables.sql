SELECT t.object_id, s.name, t.name,
       t.temporal_type, t.is_memory_optimized, t.is_filetable,
       t.is_node, t.is_edge, t.ledger_type
FROM sys.tables AS t
JOIN sys.schemas AS s ON s.schema_id = t.schema_id
WHERE s.name NOT IN ('sys', 'INFORMATION_SCHEMA', 'guest')
  AND (@p1 IS NULL OR s.name IN (SELECT [value] FROM OPENJSON(@p1)))
ORDER BY s.name, t.name;
