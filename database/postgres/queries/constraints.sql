SELECT
    con.conrelid::bigint,
    con.conname::text,
    con.contype::text,
    con.condeferrable,
    con.condeferred,
    con.convalidated,
    con.connoinherit,
    pg_catalog.pg_get_constraintdef(con.oid, true),
    pg_catalog.pg_get_expr(con.conbin, con.conrelid, true),
    COALESCE((
      SELECT pg_catalog.jsonb_agg(a.attname::text ORDER BY k.ordinal)::text
      FROM pg_catalog.unnest(con.conkey) WITH ORDINALITY AS k(attnum, ordinal)
      JOIN pg_catalog.pg_attribute AS a ON a.attrelid = con.conrelid AND a.attnum = k.attnum
      WHERE k.attnum <> 0
    ), '[]'),
    ref_ns.nspname::text,
    ref.relname::text,
    COALESCE((
      SELECT pg_catalog.jsonb_agg(a.attname::text ORDER BY k.ordinal)::text
      FROM pg_catalog.unnest(con.confkey) WITH ORDINALITY AS k(attnum, ordinal)
      JOIN pg_catalog.pg_attribute AS a ON a.attrelid = con.confrelid AND a.attnum = k.attnum
    ), '[]'),
    con.confupdtype::text,
    con.confdeltype::text,
    con.confmatchtype::text
FROM pg_catalog.pg_constraint AS con
JOIN pg_catalog.pg_class AS c ON c.oid = con.conrelid
JOIN pg_catalog.pg_namespace AS n ON n.oid = c.relnamespace
LEFT JOIN pg_catalog.pg_class AS ref ON ref.oid = con.confrelid
LEFT JOIN pg_catalog.pg_namespace AS ref_ns ON ref_ns.oid = ref.relnamespace
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
ORDER BY con.conrelid, con.conname;
