// Package diff computes typed structural changes from an expected snapshot to
// an actual snapshot. No database modifications are performed.
package diff

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/jmeiracorbal/schema-guard/schema"
	"github.com/jmeiracorbal/schema-guard/snapshot"
)

type Kind string

const (
	Added    Kind = "added"
	Removed  Kind = "removed"
	Modified Kind = "modified"
)

// Change preserves the concrete type of the changed schema object.
type Change[T any] struct {
	Kind   Kind            `json:"kind"`
	Table  schema.TableRef `json:"table"`
	Name   string          `json:"name"`
	Before *T              `json:"before,omitempty"`
	After  *T              `json:"after,omitempty"`
}

type Result struct {
	Tables      []Change[schema.Table]      `json:"tables"`
	Columns     []Change[schema.Column]     `json:"columns"`
	Indexes     []Change[schema.Index]      `json:"indexes"`
	Constraints []Change[schema.Constraint] `json:"constraints"`
}

func (r Result) Count() int {
	return len(r.Tables) + len(r.Columns) + len(r.Indexes) + len(r.Constraints)
}

func (r Result) Empty() bool { return r.Count() == 0 }

// Compare detects adds, removals and modifications. Whole-table additions or
// removals are reported once rather than as a collection of child changes.
// Cross-engine comparisons are rejected: native types are not interchangeable.
func Compare(expected, actual schema.Snapshot) (Result, error) {
	before, err := snapshot.Canonical(expected)
	if err != nil {
		return Result{}, fmt.Errorf("expected: %w", err)
	}
	after, err := snapshot.Canonical(actual)
	if err != nil {
		return Result{}, fmt.Errorf("actual: %w", err)
	}
	if before.Engine != after.Engine {
		return Result{}, fmt.Errorf("cannot compare %q with %q", before.Engine, after.Engine)
	}
	result := Result{
		Tables:      []Change[schema.Table]{},
		Columns:     []Change[schema.Column]{},
		Indexes:     []Change[schema.Index]{},
		Constraints: []Change[schema.Constraint]{},
	}
	expectedTables := make(map[schema.TableRef]schema.Table, len(before.Tables))
	actualTables := make(map[schema.TableRef]schema.Table, len(after.Tables))
	for _, t := range before.Tables {
		expectedTables[t.Ref()] = t
	}
	for _, t := range after.Tables {
		actualTables[t.Ref()] = t
	}
	refs := make([]schema.TableRef, 0, len(before.Tables)+len(after.Tables))
	seen := make(map[schema.TableRef]bool)
	for ref := range expectedTables {
		seen[ref] = true
		refs = append(refs, ref)
	}
	for ref := range actualTables {
		if !seen[ref] {
			refs = append(refs, ref)
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Schema != refs[j].Schema {
			return refs[i].Schema < refs[j].Schema
		}
		return refs[i].Name < refs[j].Name
	})
	for _, ref := range refs {
		old, hadOld := expectedTables[ref]
		newValue, hasNew := actualTables[ref]
		switch {
		case !hadOld:
			result.Tables = append(result.Tables, Change[schema.Table]{Kind: Added, Table: ref, Name: ref.Name, After: &newValue})
		case !hasNew:
			result.Tables = append(result.Tables, Change[schema.Table]{Kind: Removed, Table: ref, Name: ref.Name, Before: &old})
		default:
			result.Columns = append(result.Columns, compareNamed(ref, old.Columns, newValue.Columns, func(v schema.Column) string { return v.Name })...)
			result.Indexes = append(result.Indexes, compareNamed(ref, old.Indexes, newValue.Indexes, func(v schema.Index) string { return v.Name })...)
			result.Constraints = append(result.Constraints, compareNamed(ref, old.Constraints, newValue.Constraints, func(v schema.Constraint) string { return v.Name })...)
		}
	}
	return result, nil
}

func compareNamed[T any](table schema.TableRef, expected, actual []T, name func(T) string) []Change[T] {
	before := make(map[string]T, len(expected))
	after := make(map[string]T, len(actual))
	keys := make([]string, 0, len(expected)+len(actual))
	seen := make(map[string]bool)
	for _, v := range expected {
		key := name(v)
		before[key] = v
		seen[key] = true
		keys = append(keys, key)
	}
	for _, v := range actual {
		key := name(v)
		after[key] = v
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	changes := make([]Change[T], 0)
	for _, key := range keys {
		old, hadOld := before[key]
		newValue, hasNew := after[key]
		switch {
		case !hadOld:
			changes = append(changes, Change[T]{Kind: Added, Table: table, Name: key, After: &newValue})
		case !hasNew:
			changes = append(changes, Change[T]{Kind: Removed, Table: table, Name: key, Before: &old})
		case !reflect.DeepEqual(old, newValue):
			changes = append(changes, Change[T]{Kind: Modified, Table: table, Name: key, Before: &old, After: &newValue})
		}
	}
	return changes
}
