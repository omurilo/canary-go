package luaengine

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/opentibiabr/canary-go/internal/db"
	lua "github.com/yuin/gopher-lua"
)

type dbResultWrapper struct {
	rows    *sql.Rows
	cols    []string
	hasData bool
	values  map[string]any
}

var (
	dbResultMu     sync.RWMutex
	dbResultMap    = make(map[uint32]*dbResultWrapper)
	dbResultNextID uint32
)

// SetDB assigns the database connection wrapper to the engine.
func (e *Engine) SetDB(database *db.DB) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.database = database
}

func (e *Engine) scheduleDBCallback(fn *lua.LFunction, delay time.Duration, args ...lua.LValue) {
	e.eventMu.Lock()
	e.eventSeq++
	id := e.eventSeq
	if e.events == nil {
		e.events = make(map[int]*time.Timer)
	}
	e.events[id] = time.AfterFunc(delay, func() {
		e.eventMu.Lock()
		delete(e.events, id)
		e.eventMu.Unlock()

		e.mu.Lock()
		defer e.mu.Unlock()
		if err := e.L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, args...); err != nil {
			if e.log != nil {
				e.log.Error("async DB callback error", "err", err)
			}
		}
	})
	e.eventMu.Unlock()
}

func (e *Engine) registerDB() {
	L := e.L

	dbTbl := L.NewTable()
	L.SetField(dbTbl, "query", L.NewFunction(e.luaDBExecute))
	L.SetField(dbTbl, "asyncQuery", L.NewFunction(e.luaDBAsyncExecute))
	L.SetField(dbTbl, "storeQuery", L.NewFunction(e.luaDBStoreQuery))
	L.SetField(dbTbl, "asyncStoreQuery", L.NewFunction(e.luaDBAsyncStoreQuery))
	L.SetField(dbTbl, "escapeString", L.NewFunction(e.luaDBEscapeString))
	L.SetField(dbTbl, "escapeBlob", L.NewFunction(e.luaDBEscapeString))
	L.SetField(dbTbl, "lastInsertId", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(0))
		return 1
	}))
	L.SetField(dbTbl, "tableExists", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LTrue)
		return 1
	}))
	L.SetGlobal("db", dbTbl)

	resTbl := L.NewTable()
	L.SetField(resTbl, "getNumber", L.NewFunction(e.luaResultGetNumber))
	L.SetField(resTbl, "getString", L.NewFunction(e.luaResultGetString))
	L.SetField(resTbl, "getStream", L.NewFunction(e.luaResultGetString))
	L.SetField(resTbl, "next", L.NewFunction(e.luaResultNext))
	L.SetField(resTbl, "free", L.NewFunction(e.luaResultFree))
	L.SetGlobal("Result", resTbl)
}

func (e *Engine) luaDBExecute(L *lua.LState) int {
	query := L.CheckString(1)
	if e.database != nil && e.database.SQL != nil {
		_, err := e.database.SQL.Exec(query)
		if err != nil && e.log != nil {
			e.log.Warn("lua query failed", "err", err, "query", query[:min(len(query), 200)])
		}
		L.Push(lua.LBool(err == nil))
		return 1
	}
	L.Push(lua.LTrue) // mock success if DB not attached
	return 1
}

func (e *Engine) luaDBAsyncExecute(L *lua.LState) int {
	query := L.CheckString(1)
	cb := L.Get(2)
	go func() {
		success := true
		if e.database != nil && e.database.SQL != nil {
			_, err := e.database.SQL.Exec(query)
			success = (err == nil)
		}
		if fn, ok := cb.(*lua.LFunction); ok {
			e.scheduleDBCallback(fn, 0, lua.LBool(success))
		}
	}()
	return 0
}

func (e *Engine) luaDBStoreQuery(L *lua.LState) int {
	query := L.CheckString(1)
	if e.database != nil && e.database.SQL != nil {
		rows, err := e.database.SQL.Query(query)
		if err != nil || rows == nil {
			L.Push(lua.LFalse)
			return 1
		}
		if !rows.Next() {
			rows.Close()
			L.Push(lua.LFalse)
			return 1
		}
		cols, _ := rows.Columns()
		wrapper := &dbResultWrapper{
			rows:    rows,
			cols:    cols,
			hasData: true,
			values:  readRowValues(rows, cols),
		}
		id := atomic.AddUint32(&dbResultNextID, 1)
		dbResultMu.Lock()
		dbResultMap[id] = wrapper
		dbResultMu.Unlock()

		L.Push(lua.LNumber(id))
		return 1
	}

	L.Push(lua.LFalse)
	return 1
}

func (e *Engine) luaDBAsyncStoreQuery(L *lua.LState) int {
	query := L.CheckString(1)
	cb := L.Get(2)
	go func() {
		var resLVal lua.LValue = lua.LFalse
		if e.database != nil && e.database.SQL != nil {
			rows, err := e.database.SQL.Query(query)
			if err == nil && rows != nil && rows.Next() {
				cols, _ := rows.Columns()
				wrapper := &dbResultWrapper{
					rows:    rows,
					cols:    cols,
					hasData: true,
					values:  readRowValues(rows, cols),
				}
				id := atomic.AddUint32(&dbResultNextID, 1)
				dbResultMu.Lock()
				dbResultMap[id] = wrapper
				dbResultMu.Unlock()
				resLVal = lua.LNumber(id)
			}
		}
		if fn, ok := cb.(*lua.LFunction); ok {
			e.scheduleDBCallback(fn, 0, resLVal)
		}
	}()
	return 0
}

func (e *Engine) luaDBEscapeString(L *lua.LState) int {
	str := L.CheckString(1)
	escaped := "'" + strings.ReplaceAll(str, "'", "''") + "'"
	L.Push(lua.LString(escaped))
	return 1
}

func readRowValues(rows *sql.Rows, cols []string) map[string]any {
	vals := make([]any, len(cols))
	valPtrs := make([]any, len(cols))
	for i := range vals {
		valPtrs[i] = &vals[i]
	}
	if err := rows.Scan(valPtrs...); err != nil {
		return map[string]any{}
	}
	res := make(map[string]any, len(cols))
	for i, col := range cols {
		val := vals[i]
		if b, ok := val.([]byte); ok {
			res[col] = string(b)
		} else {
			res[col] = val
		}
	}
	return res
}

func (e *Engine) luaResultGetNumber(L *lua.LState) int {
	resID := uint32(L.CheckNumber(1))
	col := L.CheckString(2)

	dbResultMu.RLock()
	res, ok := dbResultMap[resID]
	dbResultMu.RUnlock()

	if !ok || res == nil || !res.hasData {
		L.Push(lua.LNumber(0))
		return 1
	}

	if val, found := res.values[col]; found {
		switch v := val.(type) {
		case int:
			L.Push(lua.LNumber(v))
		case int64:
			L.Push(lua.LNumber(v))
		case int32:
			L.Push(lua.LNumber(v))
		case float64:
			L.Push(lua.LNumber(v))
		case string:
			num, _ := strconv.ParseFloat(v, 64)
			L.Push(lua.LNumber(num))
		default:
			L.Push(lua.LNumber(0))
		}
		return 1
	}
	L.Push(lua.LNumber(0))
	return 1
}

func (e *Engine) luaResultGetString(L *lua.LState) int {
	resID := uint32(L.CheckNumber(1))
	col := L.CheckString(2)

	dbResultMu.RLock()
	res, ok := dbResultMap[resID]
	dbResultMu.RUnlock()

	if !ok || res == nil || !res.hasData {
		L.Push(lua.LString(""))
		return 1
	}

	if val, found := res.values[col]; found {
		L.Push(lua.LString(fmt.Sprintf("%v", val)))
		return 1
	}
	L.Push(lua.LString(""))
	return 1
}

func (e *Engine) luaResultNext(L *lua.LState) int {
	resID := uint32(L.CheckNumber(1))

	dbResultMu.Lock()
	res, ok := dbResultMap[resID]
	dbResultMu.Unlock()

	if !ok || res == nil || res.rows == nil {
		L.Push(lua.LFalse)
		return 1
	}

	if res.rows.Next() {
		res.hasData = true
		res.values = readRowValues(res.rows, res.cols)
		L.Push(lua.LTrue)
	} else {
		res.hasData = false
		res.rows.Close()
		L.Push(lua.LFalse)
	}
	return 1
}

func (e *Engine) luaResultFree(L *lua.LState) int {
	resID := uint32(L.CheckNumber(1))

	dbResultMu.Lock()
	if res, ok := dbResultMap[resID]; ok {
		if res.rows != nil {
			res.rows.Close()
		}
		delete(dbResultMap, resID)
	}
	dbResultMu.Unlock()

	return 0
}
