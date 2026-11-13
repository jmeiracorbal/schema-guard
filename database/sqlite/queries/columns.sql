SELECT cid, name, type, "notnull", dflt_value, pk, hidden
FROM pragma_table_xinfo(?, ?)
ORDER BY cid;
