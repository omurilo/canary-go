# Migration Spec: Lua Engine Parity & Missing Bindings (Paridade da Lua Engine)

## 1. Overview
This specification details remaining Lua API bindings and helper functions required in `canary-go/internal/luaengine` to achieve 100% parity with Canary C++ Lua scripts.

## 2. Target Lua Modules & Missing Bindings

### A. Wheel of Destiny (`luaengine/player.go`)
* `Player:getWheelPoints()`
* `Player:getWheelSpentPoints()`
* `Player:getWheelSpells()`
* `Player:addWheelPoints(amount)`

### B. Exaltation Forge (`luaengine/item.go` / `luaengine/player.go`)
* `Item:getTier()` / `Item:setTier(tier)`
* `Player:getForgeDust()` / `Player:addForgeDust(amount)`
* `Player:getForgeSlivers()` / `Player:addForgeSlivers(amount)`

### C. Hazard System (`luaengine/game.go` / `luaengine/zone.go`)
* `Game.getHazardLevel(position)`
* `Game.setHazardLevel(position, level)`
* `Zone.getHazardLevel()` / `Zone.setHazardLevel(level)`

### D. Advanced Imbuement Methods (`luaengine/item.go`)
* `Item:getImbuementSlots()`
* `Item:getImbuements()`
* `Item:addImbuement(imbuementId, duration)`
* `Item:removeImbuement(slot)`

### E. Client Debug Reports (`luaengine/game.go`)
* `Game.sendDebugReport(message)`
* `Player:sendClientCheck()`

## 3. Implementation Steps
1. Register missing methods in `registerPlayer`, `registerItem`, `registerGame`, and `registerZone` in `internal/luaengine/`.
2. Map Gopher-Lua values cleanly to Go struct fields with proper type checking.
3. Write unit tests in `luaengine/` for each newly registered Lua method.
