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
	Kind           string       `json:"kind,omitempty"`
	Persistence    string       `json:"persistence,omitempty"`
	Parent         *TableRef    `json:"parent,omitempty"`
	PartitionKey   *string      `json:"partition_key,omitempty"`
	PartitionBound *string      `json:"partition_bound,omitempty"`
	StorageEngine  string       `json:"storage_engine,omitempty"`
	Collation      *string      `json:"collation,omitempty"`
	RowFormat      string       `json:"row_format,omitempty"`
	CreateOptions  string       `json:"create_options,omitempty"`
	Partitions     []Partition  `json:"partitions,omitempty"`
	Schema         string       `json:"schema"`
	Name           string       `json:"name"`
	Columns        []Column     `json:"columns"`
	Indexes        []Index      `json:"indexes"`
	Constraints    []Constraint `json:"constraints"`
}

// Partition captures native partition definitions without parsing CREATE TABLE.
// Order and subpartition order are structural attributes.
type Partition struct {
	Name                   string  `json:"name"`
	Position               int     `json:"position"`
	Method                 string  `json:"method"`
	Expression             string  `json:"expression"`
	Description            *string `json:"description,omitempty"`
	Subpartition           string  `json:"subpartition,omitempty"`
	SubpartitionPosition   int     `json:"subpartition_position,omitempty"`
	SubpartitionMethod     string  `json:"subpartition_method,omitempty"`
	SubpartitionExpression string  `json:"subpartition_expression,omitempty"`
}

func (t Table) Ref() TableRef { return TableRef{Schema: t.Schema, Name: t.Name} }

type Column struct {
	Name         string  `json:"name"`
	Position     int     `json:"position"`
	DataType     string  `json:"data_type"`
	Nullable     bool    `json:"nullable"`
	Default      *string `json:"default,omitempty"`
	Generated    *string `json:"generated,omitempty"`
	Collation    *string `json:"collation,omitempty"`
	CharacterSet string  `json:"character_set,omitempty"`
	Extra        string  `json:"extra,omitempty"`
	Comment      string  `json:"comment,omitempty"`
}

type IndexColumn struct {
	// Exactly one of Name and Expression must be populated.
	Name         string `json:"name,omitempty"`
	Expression   string `json:"expression,omitempty"`
	Descending   bool   `json:"descending"`
	NullsFirst   *bool  `json:"nulls_first,omitempty"`
	PrefixLength *int   `json:"prefix_length,omitempty"`
}

type Index struct {
	Definition      *string       `json:"definition,omitempty"`
	Visibility      string        `json:"visibility,omitempty"`
	Comment         string        `json:"comment,omitempty"`
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
	NotNull    ConstraintType = "not_null"
)

type Reference struct {
	Match    string   `json:"match,omitempty"`
	Schema   string   `json:"schema"`
	Table    string   `json:"table"`
	Columns  []string `json:"columns"` // key order is significant
	OnUpdate string   `json:"on_update,omitempty"`
	OnDelete string   `json:"on_delete,omitempty"`
}

type Constraint struct {
	Definition        *string `json:"definition,omitempty"`
	InitiallyDeferred *bool   `json:"initially_deferred,omitempty"`
	Validated         *bool   `json:"validated,omitempty"`
	Enforced          *bool   `json:"enforced,omitempty"`
	NoInherit         bool    `json:"no_inherit,omitempty"`
	// Name must be a stable identifier, including for unnamed native constraints.
	Name       string         `json:"name"`
	Type       ConstraintType `json:"type"`
	Columns    []string       `json:"columns"` // key order is significant
	Expression *string        `json:"expression,omitempty"`
	References *Reference     `json:"references,omitempty"`
	Deferrable *bool          `json:"deferrable,omitempty"`
}
