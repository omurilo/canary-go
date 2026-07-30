package luaengine

import (
	lua "github.com/yuin/gopher-lua"
)

// registerWebhookType exposes the Webhook namespace.
//
// C++ registers this as a plain table plus a method (webhook_functions.cpp:16-17
// `registerTable(L, "Webhook")` / `registerMethod(L, "Webhook", "sendMessage")`),
// and both the datapack and the tests call it statically as
// `Webhook.sendMessage(...)`. It must therefore be a TABLE, not a constructor
// function — a function global fails with "attempt to index a non-table object".
//
// sendMessage is still a no-op: the C++ side posts to Discord via a curl queue
// (src/server/network/webhook/webhook.cpp), which has no Go counterpart yet.
func (e *Engine) registerWebhookType() {
	webhook := e.L.NewTable()
	e.L.SetField(webhook, "sendMessage", e.L.NewFunction(func(L *lua.LState) int {
		return 0
	}))
	e.L.SetGlobal("Webhook", webhook)
}
