SELECT ic.object_id, ic.index_id, ic.index_column_id, ic.key_ordinal,
       c.name, ic.is_descending_key, ic.is_included_column
FROM sys.index_columns AS ic
JOIN sys.indexes AS i ON i.object_id = ic.object_id AND i.index_id = ic.index_id
JOIN sys.tables AS t ON t.object_id = ic.object_id
JOIN sys.schemas AS s ON s.schema_id = t.schema_id
JOIN sys.columns AS c ON c.object_id = ic.object_id AND c.column_id = ic.column_id
WHERE ic.index_id > 0
  AND s.name NOT IN ('sys', 'INFORMATION_SCHEMA', 'guest')
  AND (@p1 IS NULL OR s.name IN (SELECT [value] FROM OPENJSON(@p1)))
ORDER BY ic.object_id, ic.index_id, ic.key_ordinal, ic.index_column_id;
