// Package cli provides the schema-guard command-line interface.
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/microsoft/go-mssqldb"
	_ "modernc.org/sqlite"

	"github.com/jmeiracorbal/schema-guard/diff"
	"github.com/jmeiracorbal/schema-guard/database/mariadb"
	"github.com/jmeiracorbal/schema-guard/database/mysql"
	"github.com/jmeiracorbal/schema-guard/database/postgres"
	"github.com/jmeiracorbal/schema-guard/schema"
	"github.com/jmeiracorbal/schema-guard/snapshot"
	"github.com/jmeiracorbal/schema-guard/database/sqlite"
	"github.com/jmeiracorbal/schema-guard/database/sqlserver"
)

const (
	ExitOK    = 0
	ExitError = 1
	ExitDrift = 2
)

type stringFlags []string

func (s *stringFlags) String() string { return strings.Join(*s, ",") }
func (s *stringFlags) Set(value string) error {
	if value == "" {
		return errors.New("schema name cannot be empty")
	}
	*s = append(*s, value)
	return nil
}

type settings struct {
	engine   string
	schemas  stringFlags
	timeout  time.Duration
	expected string
	output   string
	format   string
}

// Run returns 0 for success, 2 for structural drift and 1 for failures.
// Database credentials are read from SCHEMA_GUARD_DSN, never CLI arguments.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, getenv func(string) string, version string) int {
	if len(args) == 0 {
		printUsage(stderr)
		return ExitError
	}
	if args[0] == "version" {
		if len(args) != 1 {
			fmt.Fprintln(stderr, "version does not accept arguments")
			return ExitError
		}
		_, err := fmt.Fprintln(stdout, version)
		return exitFromError(err, stderr)
	}
	command := args[0]
	if command != "snapshot" && command != "check" {
		fmt.Fprintf(stderr, "unknown command %q\n", command)
		printUsage(stderr)
		return ExitError
	}
	cfg := settings{timeout: 2 * time.Minute, output: "-", format: "text"}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&cfg.engine, "engine", "", "database engine: postgresql, mysql, mariadb, sqlite, sqlserver")
	flags.Var(&cfg.schemas, "schema", "database schema to inspect (repeatable)")
	flags.DurationVar(&cfg.timeout, "timeout", cfg.timeout, "maximum inspection duration")
	if command == "snapshot" {
		flags.StringVar(&cfg.output, "output", cfg.output, "output snapshot path, or - for standard output")
	} else {
		flags.StringVar(&cfg.expected, "expected", "", "path to the expected schema snapshot")
		flags.StringVar(&cfg.format, "format", cfg.format, "drift report format: text or json")
	}
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ExitOK
		}
		return ExitError
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "unexpected positional arguments")
		return ExitError
	}
	if cfg.timeout <= 0 {
		fmt.Fprintln(stderr, "timeout must be greater than zero")
		return ExitError
	}
	if command == "check" && (cfg.expected == "" || (cfg.format != "text" && cfg.format != "json")) {
		fmt.Fprintln(stderr, "check requires --expected and --format text|json")
		return ExitError
	}
	engine := schema.Engine(cfg.engine)
	switch engine {
	case schema.PostgreSQL, schema.MySQL, schema.MariaDB, schema.SQLite, schema.SQLServer:
	default:
		fmt.Fprintf(stderr, "unsupported engine %q\n", engine)
		return ExitError
	}
	dsn := getenv("SCHEMA_GUARD_DSN")
	if dsn == "" {
		fmt.Fprintln(stderr, "SCHEMA_GUARD_DSN environment variable is required")
		return ExitError
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()
	actual, err := inspect(ctx, engine, dsn, []string(cfg.schemas))
	if err != nil {
		return exitFromError(err, stderr)
	}
	if command == "snapshot" {
		data, err := snapshot.Marshal(actual)
		if err != nil {
			return exitFromError(err, stderr)
		}
		data = append(data, '\n')
		if cfg.output == "-" {
			_, err = stdout.Write(data)
		} else {
			err = writeAtomic(cfg.output, data)
		}
		return exitFromError(err, stderr)
	}
	return check(actual, cfg, stdout, stderr)
}

func inspect(ctx context.Context, engine schema.Engine, dsn string, schemas []string) (schema.Snapshot, error) {
	driver := string(engine)
	switch engine {
	case schema.PostgreSQL:
		driver = "postgres"
	case schema.MariaDB:
		driver = "mysql"
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return schema.Snapshot{}, fmt.Errorf("initialize database connection: %w", err)
	}
	defer db.Close()
	if engine == schema.SQLite {
		// Named attached databases and in-memory connections are connection-local.
		db.SetMaxOpenConns(1)
	}
	if err := db.PingContext(ctx); err != nil {
		return schema.Snapshot{}, fmt.Errorf("database connectivity: %w", err)
	}
	var inspector interface {
		Inspect(context.Context) (schema.Snapshot, error)
	}
	switch engine {
	case schema.PostgreSQL:
		inspector, err = postgres.New(db, postgres.Options{Schemas: schemas})
	case schema.MySQL:
		inspector, err = mysql.New(db, mysql.Options{Schemas: schemas})
	case schema.MariaDB:
		inspector, err = mariadb.New(db, mariadb.Options{Schemas: schemas})
	case schema.SQLite:
		inspector, err = sqlite.New(db, sqlite.Options{Schemas: schemas})
	case schema.SQLServer:
		inspector, err = sqlserver.New(db, sqlserver.Options{Schemas: schemas})
	}
	if err != nil {
		return schema.Snapshot{}, fmt.Errorf("configure %s inspector: %w", engine, err)
	}
	return inspector.Inspect(ctx)
}

type report struct {
	Drift          bool        `json:"drift"`
	ChangeCount    int         `json:"change_count"`
	ExpectedSHA256 string      `json:"expected_sha256"`
	ActualSHA256   string      `json:"actual_sha256"`
	Changes        diff.Result `json:"changes"`
}

func check(actual schema.Snapshot, cfg settings, stdout, stderr io.Writer) int {
	data, err := os.ReadFile(cfg.expected)
	if err != nil {
		return exitFromError(fmt.Errorf("read expected snapshot: %w", err), stderr)
	}
	expected, err := snapshot.Unmarshal(data)
	if err != nil {
		return exitFromError(fmt.Errorf("invalid expected snapshot: %w", err), stderr)
	}
	changes, err := diff.Compare(expected, actual)
	if err != nil {
		return exitFromError(err, stderr)
	}
	before, err := snapshot.Hash(expected)
	if err != nil {
		return exitFromError(err, stderr)
	}
	after, err := snapshot.Hash(actual)
	if err != nil {
		return exitFromError(err, stderr)
	}
	result := report{Drift: !changes.Empty(), ChangeCount: changes.Count(), ExpectedSHA256: before, ActualSHA256: after, Changes: changes}
	if cfg.format == "json" {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return exitFromError(err, stderr)
		}
		if _, err := stdout.Write(append(data, '\n')); err != nil {
			return exitFromError(err, stderr)
		}
	} else if err := writeText(stdout, result); err != nil {
		return exitFromError(err, stderr)
	}
	if result.Drift {
		return ExitDrift
	}
	return ExitOK
}

func writeText(out io.Writer, result report) error {
	if !result.Drift {
		_, err := fmt.Fprintln(out, "No schema drift detected.")
		return err
	}
	if _, err := fmt.Fprintf(out, "Schema drift detected: %d change(s)\n", result.ChangeCount); err != nil {
		return err
	}
	for _, v := range result.Changes.Tables {
		if _, err := fmt.Fprintf(out, "%s table %s.%s\n", v.Kind, v.Table.Schema, v.Table.Name); err != nil {
			return err
		}
	}
	for _, v := range result.Changes.Columns {
		if _, err := fmt.Fprintf(out, "%s column %s.%s.%s\n", v.Kind, v.Table.Schema, v.Table.Name, v.Name); err != nil {
			return err
		}
	}
	for _, v := range result.Changes.Indexes {
		if _, err := fmt.Fprintf(out, "%s index %s.%s.%s\n", v.Kind, v.Table.Schema, v.Table.Name, v.Name); err != nil {
			return err
		}
	}
	for _, v := range result.Changes.Constraints {
		if _, err := fmt.Fprintf(out, "%s constraint %s.%s.%s\n", v.Kind, v.Table.Schema, v.Table.Name, v.Name); err != nil {
			return err
		}
	}
	return nil
}

func writeAtomic(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".schema-guard-*")
	if err != nil {
		return fmt.Errorf("create snapshot: %w", err)
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return fmt.Errorf("write snapshot: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync snapshot: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close snapshot: %w", err)
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("publish snapshot: %w", err)
	}
	return nil
}

func exitFromError(err error, stderr io.Writer) int {
	if err != nil {
		fmt.Fprintln(stderr, "schema-guard:", err)
		return ExitError
	}
	return ExitOK
}

func printUsage(out io.Writer) {
	fmt.Fprintln(out, "Usage: schema-guard snapshot|check|version [flags]")
	fmt.Fprintln(out, "Set SCHEMA_GUARD_DSN to the database connection string (required for snapshot/check).")
	fmt.Fprintln(out, "Use 'schema-guard snapshot -h' or 'schema-guard check -h' for command flags.")
}
