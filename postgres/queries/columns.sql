SELECT
    a.attrelid::bigint,
    a.attname::text,
    a.attnum::integer,
    pg_catalog.format_type(a.atttypid, a.atttypmod),
    NOT a.attnotnull,
    pg_catalog.pg_get_expr(ad.adbin, ad.adrelid, true),
    a.attgenerated::text,
    a.attidentity::text,
    CASE
      WHEN a.attcollation <> t.typcollation AND a.attcollation <> 0
      THEN pg_catalog.format('%I.%I', col_ns.nspname, col.collname)
      ELSE NULL
    END
FROM pg_catalog.pg_attribute AS a
JOIN pg_catalog.pg_class AS c ON c.oid = a.attrelid
JOIN pg_catalog.pg_namespace AS n ON n.oid = c.relnamespace
JOIN pg_catalog.pg_type AS t ON t.oid = a.atttypid
LEFT JOIN pg_catalog.pg_attrdef AS ad ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum
LEFT JOIN pg_catalog.pg_collation AS col ON col.oid = a.attcollation
LEFT JOIN pg_catalog.pg_namespace AS col_ns ON col_ns.oid = col.collnamespace
WHERE c.relkind IN ('r', 'p')
  AND a.attnum > 0
  AND NOT a.attisdropped
  AND LEFT(n.nspname::text, 3) <> 'pg_'
  AND n.nspname <> 'information_schema'
  AND (
    $1::jsonb IS NULL
    OR n.nspname::text IN (
      SELECT requested.name
      FROM pg_catalog.jsonb_array_elements_text($1::jsonb) AS requested(name)
    )
  )
ORDER BY a.attrelid, a.attnum;
