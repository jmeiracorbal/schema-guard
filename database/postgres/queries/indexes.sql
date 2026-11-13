SELECT
    i.indrelid::bigint,
    ic.relname::text,
    i.indisunique,
    am.amname::text,
    pg_catalog.pg_get_indexdef(i.indexrelid),
    pg_catalog.pg_get_expr(i.indpred, i.indrelid, true),
    i.indisvalid,
    i.indisready,
    i.indislive,
    COALESCE((
      SELECT pg_catalog.jsonb_agg(
        pg_catalog.jsonb_build_object(
          'name', CASE WHEN k.attnum <> 0 THEN a.attname::text ELSE '' END,
          'expression', CASE WHEN k.attnum = 0 THEN pg_catalog.pg_get_indexdef(i.indexrelid, k.ordinal::integer, true) ELSE '' END,
          'descending', CASE WHEN am.amname = 'btree' THEN (i.indoption[(k.ordinal - 1)::integer] & 1) <> 0 ELSE false END,
          'nulls_first', CASE WHEN am.amname = 'btree' THEN pg_catalog.to_jsonb((i.indoption[(k.ordinal - 1)::integer] & 2) <> 0) ELSE 'null'::jsonb END
        ) ORDER BY k.ordinal
      )::text
      FROM pg_catalog.unnest(i.indkey::smallint[]) WITH ORDINALITY AS k(attnum, ordinal)
      LEFT JOIN pg_catalog.pg_attribute AS a ON a.attrelid = i.indrelid AND a.attnum = k.attnum
      WHERE k.ordinal <= i.indnkeyatts
    ), '[]'),
    COALESCE((
      SELECT pg_catalog.jsonb_agg(a.attname::text ORDER BY k.ordinal)::text
      FROM pg_catalog.unnest(i.indkey::smallint[]) WITH ORDINALITY AS k(attnum, ordinal)
      JOIN pg_catalog.pg_attribute AS a ON a.attrelid = i.indrelid AND a.attnum = k.attnum
      WHERE k.ordinal > i.indnkeyatts
    ), '[]')
FROM pg_catalog.pg_index AS i
JOIN pg_catalog.pg_class AS c ON c.oid = i.indrelid
JOIN pg_catalog.pg_namespace AS n ON n.oid = c.relnamespace
JOIN pg_catalog.pg_class AS ic ON ic.oid = i.indexrelid
JOIN pg_catalog.pg_am AS am ON am.oid = ic.relam
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
ORDER BY i.indrelid, ic.relname;
