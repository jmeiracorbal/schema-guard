SELECT
    c.oid::bigint,
    n.nspname::text,
    c.relname::text,
    c.relkind::text,
    c.relispartition,
    c.relpersistence::text,
    parent_ns.nspname::text,
    parent.relname::text,
    pg_catalog.pg_get_partkeydef(c.oid),
    pg_catalog.pg_get_expr(c.relpartbound, c.oid, true)
FROM pg_catalog.pg_class AS c
JOIN pg_catalog.pg_namespace AS n ON n.oid = c.relnamespace
LEFT JOIN pg_catalog.pg_inherits AS inh ON inh.inhrelid = c.oid
LEFT JOIN pg_catalog.pg_class AS parent ON parent.oid = inh.inhparent
LEFT JOIN pg_catalog.pg_namespace AS parent_ns ON parent_ns.oid = parent.relnamespace
WHERE c.relkind IN ('r', 'p')
  AND LEFT(n.nspname::text, 3) <> 'pg_'
  AND n.nspname <> 'information_schema'
  AND (
    $1::jsonb IS NULL
    OR n.nspname::text IN (
      SELECT requested.name
      FROM pg_catalog.jsonb_array_elements_text($1::jsonb) AS requested(name)
    )
  )
ORDER BY n.nspname, c.relname;
