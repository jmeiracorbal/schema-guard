package schema

import "fmt"

// Validate rejects incomplete, unsupported, or ambiguous structural snapshots.
// It does not modify the snapshot.
func (s Snapshot) Validate() error {
	if s.Version != FormatVersion {
		return fmt.Errorf("snapshot version %d: supported version is %d", s.Version, FormatVersion)
	}
	switch s.Engine {
	case PostgreSQL, MySQL, MariaDB, SQLite, SQLServer:
	default:
		return fmt.Errorf("unsupported database engine %q", s.Engine)
	}
	tables := make(map[TableRef]struct{}, len(s.Tables))
	for _, t := range s.Tables {
		if t.Name == "" {
			return fmt.Errorf("table name must not be empty")
		}
		ref := t.Ref()
		if _, exists := tables[ref]; exists {
			return fmt.Errorf("duplicate table %q in schema %q", t.Name, t.Schema)
		}
		tables[ref] = struct{}{}
		if err := t.validate(); err != nil {
			return fmt.Errorf("table %q (schema %q): %w", t.Name, t.Schema, err)
		}
	}
	return nil
}

func (t Table) validate() error {
	partitionNames := make(map[string]struct{}, len(t.Partitions))
	partitionPositions := make(map[int]struct{}, len(t.Partitions))
	for _, p := range t.Partitions {
		if p.Name == "" || p.Method == "" || p.Position <= 0 {
			return fmt.Errorf("partition must have a name, method and positive position")
		}
		key := p.Name + "\x00" + p.Subpartition
		if _, ok := partitionNames[key]; ok {
			return fmt.Errorf("duplicate partition %q", key)
		}
		partitionNames[key] = struct{}{}
		number := p.Position*1000000 + p.SubpartitionPosition
		if _, ok := partitionPositions[number]; ok {
			return fmt.Errorf("duplicate partition position for %q", key)
		}
		partitionPositions[number] = struct{}{}
	}

	columns := make(map[string]struct{}, len(t.Columns))
	positions := make(map[int]struct{}, len(t.Columns))
	for _, c := range t.Columns {
		if c.Name == "" || c.DataType == "" || c.Position <= 0 {
			return fmt.Errorf("column %q must have a name, data type and positive position", c.Name)
		}
		if _, ok := columns[c.Name]; ok {
			return fmt.Errorf("duplicate column %q", c.Name)
		}
		if _, ok := positions[c.Position]; ok {
			return fmt.Errorf("duplicate column position %d", c.Position)
		}
		columns[c.Name] = struct{}{}
		positions[c.Position] = struct{}{}
	}
	indexes := make(map[string]struct{}, len(t.Indexes))
	for _, i := range t.Indexes {
		if i.Name == "" || len(i.Columns) == 0 {
			return fmt.Errorf("index %q must have a name and at least one key", i.Name)
		}
		if _, ok := indexes[i.Name]; ok {
			return fmt.Errorf("duplicate index %q", i.Name)
		}
		indexes[i.Name] = struct{}{}
		for _, key := range i.Columns {
			if key.PrefixLength != nil && *key.PrefixLength <= 0 {
				return fmt.Errorf("index %q has a nonpositive prefix length", i.Name)
			}
			if (key.Name == "") == (key.Expression == "") {
				return fmt.Errorf("index %q key must have exactly one of name or expression", i.Name)
			}
		}
		included := make(map[string]struct{}, len(i.IncludedColumns))
		for _, col := range i.IncludedColumns {
			if col == "" {
				return fmt.Errorf("index %q includes an empty column name", i.Name)
			}
			if _, ok := included[col]; ok {
				return fmt.Errorf("index %q includes duplicate column %q", i.Name, col)
			}
			included[col] = struct{}{}
		}
	}
	constraints := make(map[string]struct{}, len(t.Constraints))
	for _, c := range t.Constraints {
		if c.Name == "" {
			return fmt.Errorf("constraint name must not be empty")
		}
		if _, ok := constraints[c.Name]; ok {
			return fmt.Errorf("duplicate constraint %q", c.Name)
		}
		constraints[c.Name] = struct{}{}
		switch c.Type {
		case PrimaryKey, UniqueKey:
			if len(c.Columns) == 0 || c.References != nil || c.Expression != nil {
				return fmt.Errorf("constraint %q: invalid primary/unique definition", c.Name)
			}
		case ForeignKey:
			if c.References == nil || c.References.Table == "" || len(c.Columns) == 0 || len(c.Columns) != len(c.References.Columns) || c.Expression != nil {
				return fmt.Errorf("constraint %q: invalid foreign key definition", c.Name)
			}
		case NotNull:
			if len(c.Columns) != 1 || c.References != nil || c.Expression != nil {
				return fmt.Errorf("constraint %q: invalid not-null definition", c.Name)
			}
		case Check:
			if c.Expression == nil || *c.Expression == "" || c.References != nil {
				return fmt.Errorf("constraint %q: invalid check definition", c.Name)
			}
		case Exclusion:
			if (len(c.Columns) == 0 && c.Expression == nil) || c.References != nil {
				return fmt.Errorf("constraint %q: invalid exclusion definition", c.Name)
			}
		default:
			return fmt.Errorf("constraint %q: unsupported kind %q", c.Name, c.Type)
		}
		for _, col := range c.Columns {
			if col == "" {
				return fmt.Errorf("constraint %q: empty column name", c.Name)
			}
		}
		if c.References != nil {
			for _, col := range c.References.Columns {
				if col == "" {
					return fmt.Errorf("constraint %q: empty referenced column name", c.Name)
				}
			}
		}
	}
	return nil
}
