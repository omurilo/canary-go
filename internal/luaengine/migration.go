package luaengine

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

// RunMigration executes one database migration script, porting the per-script
// half of DatabaseManager::updateDatabase (src/database/databasemanager.cpp:113):
// load the file, then call the global onUpdateDatabase().
//
// C++ builds a throwaway lua_State with only luaL_openlibs + CoreLibsFunctions
// for migrations, so the scripts see just db, logger and the standard library.
// Here they run on the main engine state instead, which is a superset — the
// migration scripts in data-otservbr-global only use db.query, db.tableExists,
// db.storeQuery and logger.*, all of which are registered.
//
// The boolean result is read but intentionally not enforced: C++ advances
// db_version whenever the call itself succeeds, regardless of what the script
// returns. Returning it here would change which migrations count as applied, so
// it is only surfaced in the error message when the script explicitly fails.
func (e *Engine) RunMigration(path string) error {
	if err := e.DoFile(path); err != nil {
		return fmt.Errorf("load migration: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	fn := e.L.GetGlobal("onUpdateDatabase")
	if fn.Type() != lua.LTFunction {
		return fmt.Errorf("migration has no onUpdateDatabase function")
	}

	if err := e.L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}); err != nil {
		return fmt.Errorf("onUpdateDatabase: %w", err)
	}
	e.L.Pop(1)

	// Clear the global so the next migration cannot silently inherit this one's
	// function if its own file fails to define one.
	e.L.SetGlobal("onUpdateDatabase", lua.LNil)
	return nil
}
