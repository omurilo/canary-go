package luaengine

import "testing"

// TestRunShutdownGlobalEvents covers the hireling-persistence bug: ExecuteShutdown
// (globalevents.go:139) was never called from main.go, so the hireling save that
// lives on onShutdown (hireling_save.lua → SaveHirelings) never ran, and spawned
// hirelings' active flag/position were lost on restart. RunShutdownGlobalEvents
// must fire onShutdown callbacks.
func TestRunShutdownGlobalEvents(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	if err := e.L.DoString(`
		local ge = GlobalEvent("ShutdownTest")
		ge:type("shutdown")
		shutdownFired = false
		function ge.onShutdown()
			shutdownFired = true
			return true
		end
		ge:register()
	`); err != nil {
		t.Fatalf("registering shutdown global event: %v", err)
	}

	e.RunShutdownGlobalEvents()

	if err := e.L.DoString(`
		assert(shutdownFired == true, "onShutdown callback was not fired")
	`); err != nil {
		t.Fatalf("RunShutdownGlobalEvents did not fire onShutdown: %v", err)
	}
}
