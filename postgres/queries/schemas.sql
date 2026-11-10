SELECT n.nspname::text
FROM pg_catalog.pg_namespace AS n
WHERE LEFT(n.nspname::text, 3) <> 'pg_'
  AND n.nspname <> 'information_schema'
  AND (
    $1::jsonb IS NULL
    OR n.nspname::text IN (
      SELECT requested.name
      FROM pg_catalog.jsonb_array_elements_text($1::jsonb) AS requested(name)
    )
  )
ORDER BY n.nspname;
