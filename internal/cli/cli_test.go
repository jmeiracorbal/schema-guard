package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmeiracorbal/schema-guard/schema"
	"github.com/jmeiracorbal/schema-guard/snapshot"
)

func invoke(args []string, dsn string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	status := Run(context.Background(), args, &stdout, &stderr, func(k string) string {
		if k == "SCHEMA_GUARD_DSN" {
			return dsn
		}
		return ""
	}, "test-version")
	return status, stdout.String(), stderr.String()
}

func TestArguments(t *testing.T) {
	cases := []struct {
		args []string
		want int
	}{
		{[]string{}, ExitError},
		{[]string{"unknown"}, ExitError},
		{[]string{"version"}, ExitOK},
		{[]string{"version", "extra"}, ExitError},
		{[]string{"snapshot", "--engine", "other"}, ExitError},
		{[]string{"snapshot", "--engine", "sqlite"}, ExitError},
		{[]string{"snapshot", "--engine", "sqlite", "--timeout", "0s"}, ExitError},
		{[]string{"check", "--engine", "sqlite"}, ExitError},
		{[]string{"snapshot", "--engine", "sqlite", "--schema", ""}, ExitError},
		{[]string{"snapshot", "-h"}, ExitOK},
	}
	for _, c := range cases {
		got, _, _ := invoke(c.args, "")
		if got != c.want {
			t.Errorf("%q: got %d, want %d", c.args, got, c.want)
		}
	}
}

func TestSnapshotCheckAndDrift(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.sqlite")
	dsn := "file:" + filepath.ToSlash(path) + "?mode=ro"
	writer, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if _, err := writer.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, label TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(t.TempDir(), "expected.json")
	status, _, errors := invoke([]string{"snapshot", "--engine", "sqlite", "--output", expected}, dsn)
	if status != ExitOK {
		t.Fatalf("snapshot failed (%d): %s", status, errors)
	}
	data, err := os.ReadFile(expected)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot schema.Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil || len(snapshot.Tables) != 1 {
		t.Fatalf("invalid snapshot: %v, %+v", err, snapshot)
	}
	if stat, err := os.Stat(expected); err != nil || stat.Mode().Perm() != 0600 {
		t.Fatalf("snapshot permissions: %v, %v", stat, err)
	}
	status, out, errors := invoke([]string{"check", "--engine", "sqlite", "--expected", expected, "--format", "json"}, dsn)
	if status != ExitOK || errors != "" {
		t.Fatalf("check without drift (%d): %s", status, errors)
	}
	var r report
	if err := json.Unmarshal([]byte(out), &r); err != nil || r.Drift || r.ChangeCount != 0 || r.ExpectedSHA256 != r.ActualSHA256 {
		t.Fatalf("unexpected report: %v, %+v", err, r)
	}
	if _, err := writer.Exec("ALTER TABLE items ADD COLUMN observed_only INTEGER"); err != nil {
		t.Fatal(err)
	}
	status, out, errors = invoke([]string{"check", "--engine", "sqlite", "--expected", expected}, dsn)
	if status != ExitDrift || !strings.Contains(out, "drift detected") || errors != "" {
		t.Fatalf("drift check (%d): %s, %s", status, out, errors)
	}
	status, out, _ = invoke([]string{"check", "--engine", "sqlite", "--expected", expected, "--format", "json"}, dsn)
	if status != ExitDrift || json.Unmarshal([]byte(out), &r) != nil || !r.Drift || r.ChangeCount == 0 || r.ActualSHA256 == r.ExpectedSHA256 {
		t.Fatalf("unexpected drift json (%d): %s", status, out)
	}
}

func TestVersionAndMalformedSnapshot(t *testing.T) {
	status, out, _ := invoke([]string{"version"}, "")
	if status != ExitOK || out != "test-version\n" {
		t.Fatalf("version: %d, %q", status, out)
	}
	file := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(file, []byte(`{"version":1,"engine":"mysql","tables":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	status, _, stderr := invoke([]string{"check", "--engine", "sqlite", "--expected", file}, "file::memory:")
	if status != ExitError || !strings.Contains(stderr, "cannot compare") {
		t.Fatalf("cross-engine check: %d, %s", status, stderr)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func TestCheckPropagatesJSONWriteError(t *testing.T) {
	actual := schema.Snapshot{
		Version: schema.FormatVersion,
		Engine:  schema.SQLite,
		Tables:  []schema.Table{},
	}
	expected, err := snapshot.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "expected.json")
	if err := os.WriteFile(path, expected, 0600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	status := check(actual, settings{expected: path, format: "json"}, failingWriter{}, &stderr)
	if status != ExitError || !strings.Contains(stderr.String(), io.ErrClosedPipe.Error()) {
		t.Fatalf("expected propagated output error; status=%d stderr=%q", status, stderr.String())
	}
}
