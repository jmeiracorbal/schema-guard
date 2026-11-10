// Package snapshot validates and serializes deterministic structural snapshots.
package snapshot

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/jmeiracorbal/schema-guard/schema"
)

// Canonical returns a deep copy of the structural collections, sorted wherever
// order has no semantic significance. Meaningful ordering (column positions,
// index keys, constraint keys and referenced keys) is preserved.
func Canonical(input schema.Snapshot) (schema.Snapshot, error) {
	if err := input.Validate(); err != nil {
		return schema.Snapshot{}, err
	}
	result := schema.Snapshot{
		Version: input.Version,
		Engine:  input.Engine,
		Tables:  make([]schema.Table, 0, len(input.Tables)),
	}
	for _, t := range input.Tables {
		copyTable := schema.Table{
			Kind:           t.Kind,
			Persistence:    t.Persistence,
			Parent:         t.Parent,
			PartitionKey:   t.PartitionKey,
			PartitionBound: t.PartitionBound,
			Schema:         t.Schema,
			Name:           t.Name,
			Columns:        append(make([]schema.Column, 0, len(t.Columns)), t.Columns...),
			Indexes:        make([]schema.Index, 0, len(t.Indexes)),
			Constraints:    make([]schema.Constraint, 0, len(t.Constraints)),
		}
		sort.Slice(copyTable.Columns, func(i, j int) bool {
			return copyTable.Columns[i].Position < copyTable.Columns[j].Position
		})
		for _, idx := range t.Indexes {
			idx.Columns = append(make([]schema.IndexColumn, 0, len(idx.Columns)), idx.Columns...)
			idx.IncludedColumns = append(make([]string, 0, len(idx.IncludedColumns)), idx.IncludedColumns...)
			sort.Strings(idx.IncludedColumns)
			copyTable.Indexes = append(copyTable.Indexes, idx)
		}
		sort.Slice(copyTable.Indexes, func(i, j int) bool {
			return copyTable.Indexes[i].Name < copyTable.Indexes[j].Name
		})
		for _, constraint := range t.Constraints {
			constraint.Columns = append(make([]string, 0, len(constraint.Columns)), constraint.Columns...)
			if constraint.References != nil {
				ref := *constraint.References
				ref.Columns = append(make([]string, 0, len(ref.Columns)), ref.Columns...)
				constraint.References = &ref
			}
			copyTable.Constraints = append(copyTable.Constraints, constraint)
		}
		sort.Slice(copyTable.Constraints, func(i, j int) bool {
			return copyTable.Constraints[i].Name < copyTable.Constraints[j].Name
		})
		result.Tables = append(result.Tables, copyTable)
	}
	sort.Slice(result.Tables, func(i, j int) bool {
		if result.Tables[i].Schema != result.Tables[j].Schema {
			return result.Tables[i].Schema < result.Tables[j].Schema
		}
		return result.Tables[i].Name < result.Tables[j].Name
	})
	return result, nil
}

// CanonicalJSON produces compact, deterministic JSON for hashing or storage.
func CanonicalJSON(input schema.Snapshot) ([]byte, error) {
	canonical, err := Canonical(input)
	if err != nil {
		return nil, err
	}
	return json.Marshal(canonical)
}

// Marshal returns human-readable JSON of a validated canonical snapshot.
func Marshal(input schema.Snapshot) ([]byte, error) {
	canonical, err := Canonical(input)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(canonical, "", "  ")
}

// Unmarshal rejects unknown fields, extra JSON values and unsupported formats.
func Unmarshal(data []byte) (schema.Snapshot, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var s schema.Snapshot
	if err := decoder.Decode(&s); err != nil {
		return schema.Snapshot{}, fmt.Errorf("decode snapshot: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return schema.Snapshot{}, fmt.Errorf("decode snapshot: unexpected trailing JSON value")
		}
		return schema.Snapshot{}, fmt.Errorf("decode snapshot: %w", err)
	}
	return Canonical(s)
}

// Hash computes SHA-256 of canonical JSON and returns its lowercase hex digest.
// Connection information and capture timestamps are not part of the model.
func Hash(input schema.Snapshot) (string, error) {
	data, err := CanonicalJSON(input)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
