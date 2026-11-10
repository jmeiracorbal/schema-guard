SELECT s.name
FROM sys.schemas AS s
WHERE s.name NOT IN ('sys', 'INFORMATION_SCHEMA', 'guest')
ORDER BY s.name;
