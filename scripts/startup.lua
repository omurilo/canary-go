-- Starter Lua script proving the scripting bridge works end to end.
-- The Go server invokes these global hooks; the existing Canary API surface is
-- ported incrementally on top of this same pattern.

logger.info("[lua] startup.lua loaded — scripting engine is alive")

function onPlayerLogin(name)
    logger.info("[lua] player logged in: " .. tostring(name))
end

function onPlayerSay(name, text)
    logger.info("[lua] " .. tostring(name) .. " says: " .. tostring(text))
end
