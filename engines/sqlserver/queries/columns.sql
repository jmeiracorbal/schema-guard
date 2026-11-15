SELECT c.object_id, c.name, c.column_id,
       CASE
         WHEN ty.is_user_defined = 1 OR ty.is_assembly_type = 1
           THEN CONCAT(QUOTENAME(ts.name), '.', QUOTENAME(ty.name))
         WHEN ty.name IN ('nvarchar','nchar')
           THEN CONCAT(ty.name, '(', CASE WHEN c.max_length = -1 THEN 'max' ELSE CAST(c.max_length/2 AS VARCHAR(10)) END, ')')
         WHEN ty.name IN ('varchar','char','varbinary','binary')
           THEN CONCAT(ty.name, '(', CASE WHEN c.max_length = -1 THEN 'max' ELSE CAST(c.max_length AS VARCHAR(10)) END, ')')
         WHEN ty.name IN ('decimal','numeric')
           THEN CONCAT(ty.name, '(', c.precision, ',', c.scale, ')')
         WHEN ty.name IN ('datetime2','datetimeoffset','time')
           THEN CONCAT(ty.name, '(', c.scale, ')')
         WHEN ty.name = 'float'
           THEN CONCAT(ty.name, '(', c.precision, ')')
         ELSE ty.name
       END,
       c.is_nullable, dc.definition, cc.definition, cc.is_persisted,
       CAST(ic.seed_value AS NVARCHAR(100)), CAST(ic.increment_value AS NVARCHAR(100)), ic.is_not_for_replication,
       c.collation_name, ty.is_user_defined, ty.is_assembly_type,
       c.is_hidden, c.is_sparse, c.is_column_set, c.is_filestream,
       c.generated_always_type, c.encryption_type
FROM sys.columns AS c
JOIN sys.tables AS t ON t.object_id = c.object_id
JOIN sys.schemas AS s ON s.schema_id = t.schema_id
JOIN sys.types AS ty ON ty.user_type_id = c.user_type_id
JOIN sys.schemas AS ts ON ts.schema_id = ty.schema_id
LEFT JOIN sys.default_constraints AS dc ON dc.object_id = c.default_object_id
LEFT JOIN sys.computed_columns AS cc ON cc.object_id = c.object_id AND cc.column_id = c.column_id
LEFT JOIN sys.identity_columns AS ic ON ic.object_id = c.object_id AND ic.column_id = c.column_id
WHERE s.name NOT IN ('sys', 'INFORMATION_SCHEMA', 'guest')
  AND (@p1 IS NULL OR s.name IN (SELECT [value] FROM OPENJSON(@p1)))
ORDER BY c.object_id, c.column_id;
