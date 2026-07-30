package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
)

// migrationFilePattern mirrors InternalDBManager::extractVersionFromFilename
// (src/database/databasemanager.cpp:17): the regex is `(\d+)\.lua`, and a name
// that does not match yields -1 — which is how README.md in the migrations
// directory ends up skipped.
var migrationFilePattern = regexp.MustCompile(`(\d+)\.lua`)

// Migration is one migration script on disk.
type Migration struct {
	Version int
	Path    string
}

// MigrationRunner runs a single migration script. The Lua engine implements it:
// dofile the script, then call the global onUpdateDatabase().
type MigrationRunner interface {
	RunMigration(path string) error
}

// extractMigrationVersion returns the version encoded in a migration filename,
// or -1 when the name carries none.
func extractMigrationVersion(filename string) int {
	m := migrationFilePattern.FindStringSubmatch(filename)
	if m == nil {
		return -1
	}
	v, err := strconv.Atoi(m[1])
	if err != nil {
		return -1
	}
	return v
}

// DatabaseVersion ports DatabaseManager::getDatabaseVersion
// (src/database/databasemanager.cpp:73). When server_config is missing it is
// created and seeded with db_version = 0, so a fresh database starts at 0 rather
// than erroring.
func (d *DB) DatabaseVersion(ctx context.Context) (int, error) {
	exists, err := d.tableExists(ctx, "server_config")
	if err != nil {
		return 0, err
	}
	if !exists {
		const create = "CREATE TABLE `server_config` (" +
			"`config` VARCHAR(50) NOT NULL, " +
			"`value` VARCHAR(256) NOT NULL DEFAULT '', " +
			"UNIQUE(`config`)) ENGINE = InnoDB"
		if _, err := d.SQL.ExecContext(ctx, create); err != nil {
			return 0, fmt.Errorf("db: create server_config: %w", err)
		}
		if _, err := d.SQL.ExecContext(ctx, "INSERT INTO `server_config` VALUES ('db_version', 0)"); err != nil {
			return 0, fmt.Errorf("db: seed db_version: %w", err)
		}
		return 0, nil
	}

	var raw string
	err = d.SQL.QueryRowContext(ctx,
		"SELECT `value` FROM `server_config` WHERE `config` = 'db_version'").Scan(&raw)
	if err != nil {
		// C++ returns -1 when the row cannot be read, which makes every migration
		// eligible. Mirror that rather than silently starting from 0.
		return -1, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return -1, nil
	}
	return v, nil
}

// setDatabaseVersion ports registerDatabaseConfig("db_version", v).
func (d *DB) setDatabaseVersion(ctx context.Context, v int) error {
	_, err := d.SQL.ExecContext(ctx,
		"INSERT INTO `server_config` (`config`, `value`) VALUES ('db_version', ?) "+
			"ON DUPLICATE KEY UPDATE `value` = VALUES(`value`)", strconv.Itoa(v))
	if err != nil {
		return fmt.Errorf("db: set db_version: %w", err)
	}
	return nil
}

// tableExists reports whether a table is present in the configured database.
func (d *DB) tableExists(ctx context.Context, name string) (bool, error) {
	var n int
	err := d.SQL.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM information_schema.tables "+
			"WHERE table_schema = DATABASE() AND table_name = ?", name).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("db: table_exists %s: %w", name, err)
	}
	return n > 0, nil
}

// CollectMigrations lists the migration scripts in dir, sorted by version, the
// way updateDatabase does before iterating.
func CollectMigrations(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("db: read migrations dir %s: %w", dir, err)
	}

	migrations := make([]Migration, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		migrations = append(migrations, Migration{
			Version: extractMigrationVersion(e.Name()),
			Path:    filepath.Join(dir, e.Name()),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		if migrations[i].Version != migrations[j].Version {
			return migrations[i].Version < migrations[j].Version
		}
		return migrations[i].Path < migrations[j].Path
	})
	return migrations, nil
}

// UpdateDatabase ports DatabaseManager::updateDatabase
// (src/database/databasemanager.cpp:88). It runs every migration whose version is
// greater than the stored db_version, in ascending order, bumping db_version after
// each one.
//
// Two behaviours are deliberately copied from C++ rather than improved:
//   - a script whose Lua call fails is logged and SKIPPED, and db_version is not
//     advanced past it, but iteration continues with the next script;
//   - the boolean that onUpdateDatabase returns is NOT inspected. C++ calls it with
//     lua_pcall(L, 0, 1, 0) and then advances the version regardless of the result,
//     so a migration that returns false still counts as applied.
//
// It returns the version the database ended on.
func (d *DB) UpdateDatabase(ctx context.Context, dir string, runner MigrationRunner, log Logger) (int, error) {
	current, err := d.DatabaseVersion(ctx)
	if err != nil {
		return 0, err
	}

	migrations, err := CollectMigrations(dir)
	if err != nil {
		return current, err
	}

	for _, m := range migrations {
		if m.Version <= current {
			continue
		}
		if err := runner.RunMigration(m.Path); err != nil {
			log.Warn("migration failed", "version", m.Version, "path", m.Path, "err", err)
			continue
		}
		current = m.Version
		if err := d.setDatabaseVersion(ctx, current); err != nil {
			return current, err
		}
		log.Info("database has been updated", "version", current)
	}

	return current, nil
}

// Logger is the small slice of *slog.Logger that UpdateDatabase needs.
type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
}
