SELECT fk.parent_object_id, fk.name, fkc.constraint_column_id,
       parent_col.name, ref_schema.name, ref_table.name, ref_col.name,
       fk.update_referential_action_desc, fk.delete_referential_action_desc,
       fk.is_disabled, fk.is_not_trusted, fk.is_not_for_replication
FROM sys.foreign_keys AS fk
JOIN sys.tables AS t ON t.object_id = fk.parent_object_id
JOIN sys.schemas AS s ON s.schema_id = t.schema_id
JOIN sys.foreign_key_columns AS fkc ON fkc.constraint_object_id = fk.object_id
JOIN sys.columns AS parent_col ON parent_col.object_id = fkc.parent_object_id AND parent_col.column_id = fkc.parent_column_id
JOIN sys.tables AS ref_table ON ref_table.object_id = fk.referenced_object_id
JOIN sys.schemas AS ref_schema ON ref_schema.schema_id = ref_table.schema_id
JOIN sys.columns AS ref_col ON ref_col.object_id = fkc.referenced_object_id AND ref_col.column_id = fkc.referenced_column_id
WHERE s.name NOT IN ('sys', 'INFORMATION_SCHEMA', 'guest')
  AND (@p1 IS NULL OR s.name IN (SELECT [value] FROM OPENJSON(@p1)))
ORDER BY fk.parent_object_id, fk.name, fkc.constraint_column_id;
