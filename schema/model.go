// Package schema defines the database-independent structural model.
// Engine-specific types and expressions are preserved without lossy translation.
package schema

// FormatVersion is the current snapshot schema version.
const FormatVersion = 1

type Engine string

const (
	PostgreSQL Engine = "postgresql"
	MySQL      Engine = "mysql"
	MariaDB    Engine = "mariadb"
	SQLite     Engine = "sqlite"
	SQLServer  Engine = "sqlserver"
)

type Snapshot struct {
	Version int     `json:"version"`
	Engine  Engine  `json:"engine"`
	Tables  []Table `json:"tables"`
}

type TableRef struct {
	Schema string `json:"schema"`
	Name   string `json:"name"`
}

type Table struct {
	Schema      string       `json:"schema"`
	Name        string       `json:"name"`
	Columns     []Column     `json:"columns"`
	Indexes     []Index      `json:"indexes"`
	Constraints []Constraint `json:"constraints"`
}

func (t Table) Ref() TableRef { return TableRef{Schema: t.Schema, Name: t.Name} }

type Column struct {
	Name      string  `json:"name"`
	Position  int     `json:"position"`
	DataType  string  `json:"data_type"`
	Nullable  bool    `json:"nullable"`
	Default   *string `json:"default,omitempty"`
	Generated *string `json:"generated,omitempty"`
	Collation *string `json:"collation,omitempty"`
}

type IndexColumn struct {
	// Exactly one of Name and Expression must be populated.
	Name       string `json:"name,omitempty"`
	Expression string `json:"expression,omitempty"`
	Descending bool   `json:"descending"`
	NullsFirst *bool  `json:"nulls_first,omitempty"`
}

type Index struct {
	Name            string        `json:"name"`
	Unique          bool          `json:"unique"`
	Method          string        `json:"method"`
	Columns         []IndexColumn `json:"columns"` // key order is significant
	IncludedColumns []string      `json:"included_columns"`
	Predicate       *string       `json:"predicate,omitempty"`
}

type ConstraintType string

const (
	PrimaryKey ConstraintType = "primary_key"
	ForeignKey ConstraintType = "foreign_key"
	UniqueKey  ConstraintType = "unique"
	Check      ConstraintType = "check"
	Exclusion  ConstraintType = "exclusion"
)

type Reference struct {
	Schema   string   `json:"schema"`
	Table    string   `json:"table"`
	Columns  []string `json:"columns"` // key order is significant
	OnUpdate string   `json:"on_update,omitempty"`
	OnDelete string   `json:"on_delete,omitempty"`
}

type Constraint struct {
	// Name must be a stable identifier, including for unnamed native constraints.
	Name       string         `json:"name"`
	Type       ConstraintType `json:"type"`
	Columns    []string       `json:"columns"` // key order is significant
	Expression *string        `json:"expression,omitempty"`
	References *Reference     `json:"references,omitempty"`
	Deferrable *bool          `json:"deferrable,omitempty"`
}
