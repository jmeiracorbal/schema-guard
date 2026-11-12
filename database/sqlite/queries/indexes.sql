SELECT name, "unique", origin, partial
FROM pragma_index_list(?, ?)
ORDER BY name;
