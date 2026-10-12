package tbb

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// Migration is one versioned, forward-only schema change. Up runs inside a
// transaction together with the bookkeeping row, so a failed migration leaves
// no trace.
type Migration struct {
	Version int
	Name    string
	Up      func(ctx context.Context, tx bun.Tx) error
}

// schemaVersion records the framework migrations that have been applied.
// It is separate from whatever migration tool the app uses for its own
// tables.
type schemaVersion struct {
	bun.BaseModel `bun:"table:tbb_schema"`

	Version   int       `bun:",pk"`
	Name      string    `bun:",notnull"`
	AppliedAt time.Time `bun:",notnull"`
}

// frameworkMigrations is the framework's schema history. Append new entries
// with increasing versions; never edit an entry that has been released.
var frameworkMigrations = []Migration{
	{
		Version: 1,
		Name:    "create accounts and sessions",
		Up: func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.NewCreateTable().Model((*Account)(nil)).IfNotExists().Exec(ctx); err != nil {
				return err
			}
			_, err := tx.NewCreateTable().Model((*sessionRecord)(nil)).IfNotExists().Exec(ctx)
			return err
		},
	},
}

func migrate(ctx context.Context, db *bun.DB, migrations []Migration) error {
	if _, err := db.NewCreateTable().Model((*schemaVersion)(nil)).IfNotExists().Exec(ctx); err != nil {
		return fmt.Errorf("create schema table: %w", err)
	}

	var applied []int
	if err := db.NewSelect().Model((*schemaVersion)(nil)).Column("version").Scan(ctx, &applied); err != nil {
		return fmt.Errorf("read schema versions: %w", err)
	}
	done := make(map[int]bool, len(applied))
	for _, v := range applied {
		done[v] = true
	}

	last := 0
	for _, m := range migrations {
		if m.Version <= last {
			return fmt.Errorf("migration %d (%s) is out of order", m.Version, m.Name)
		}
		last = m.Version
		if done[m.Version] {
			continue
		}
		err := db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if err := m.Up(ctx, tx); err != nil {
				return err
			}
			_, err := tx.NewInsert().Model(&schemaVersion{
				Version:   m.Version,
				Name:      m.Name,
				AppliedAt: time.Now().UTC(),
			}).Exec(ctx)
			return err
		})
		if err != nil {
			return fmt.Errorf("migration %d (%s): %w", m.Version, m.Name, err)
		}
	}
	return nil
}
