SELECT name, type, wr, strict
FROM pragma_table_list
WHERE schema = ?
  AND name NOT LIKE 'sqlite_%'
ORDER BY name;
