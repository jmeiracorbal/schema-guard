SELECT i.object_id, i.index_id, i.name, i.is_unique, i.type, i.type_desc,
       i.filter_definition, i.is_disabled, i.is_hypothetical,
       i.fill_factor, i.is_padded, i.ignore_dup_key,
       i.allow_row_locks, i.allow_page_locks
FROM sys.indexes AS i
JOIN sys.tables AS t ON t.object_id = i.object_id
JOIN sys.schemas AS s ON s.schema_id = t.schema_id
WHERE i.index_id > 0
  AND s.name NOT IN ('sys', 'INFORMATION_SCHEMA', 'guest')
  AND (@p1 IS NULL OR s.name IN (SELECT [value] FROM OPENJSON(@p1)))
ORDER BY i.object_id, i.index_id;
