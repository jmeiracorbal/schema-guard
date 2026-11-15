SELECT ck.parent_object_id, ck.name, ck.definition,
       ck.is_disabled, ck.is_not_trusted, ck.is_not_for_replication,
       ck.parent_column_id, c.name
FROM sys.check_constraints AS ck
JOIN sys.tables AS t ON t.object_id = ck.parent_object_id
JOIN sys.schemas AS s ON s.schema_id = t.schema_id
LEFT JOIN sys.columns AS c ON c.object_id = ck.parent_object_id AND c.column_id = ck.parent_column_id
WHERE s.name NOT IN ('sys', 'INFORMATION_SCHEMA', 'guest')
  AND (@p1 IS NULL OR s.name IN (SELECT [value] FROM OPENJSON(@p1)))
ORDER BY ck.parent_object_id, ck.name;
