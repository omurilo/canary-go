# Migration Spec: Wheel of Destiny (Roda do Destino)

## 1. Overview
The Wheel of Destiny is a character progression tree introduced in Tibia 13.10. Players earn promotion points as they gain levels above 51 and spend them on a 4-domain skill wheel (Sped, Mastery, Combat, and Resilience) to unlock passive perks, attribute boosts, and major spell enhancements.

## 2. C++ Source References
* `src/creatures/players/wheel/player_wheel.hpp`
* `src/creatures/players/wheel/player_wheel.cpp`
* `src/server/network/protocol/protocolgame.cpp` (Opcodes `0xD6` Wheel Open/Save)

## 3. Go Target Packages
* `canary-go/internal/game/wheel.go`
* `canary-go/internal/protocol/game_encode.go` (Wheel network protocol)
* `canary-go/internal/luaengine/player.go` (Lua bindings)

## 4. Database Schema
* Table: `player_wheel`
  * `player_id` (INT)
  * `slot_points` (TEXT / BLOB - active point distribution)
  * `preset_id` (INT - current active preset)

## 5. Protocol Opcodes
* Client → Server: `0xEC` (Open Wheel / Save Wheel Preset)
* Server → Client: `0xD6` (Wheel Data Payload: points available, domain allocations, active perks, preset configurations)

## 6. Functional Requirements
1. **Point Calculation:** Allocate 1 Wheel Point per level above level 50 for promoted vocations.
2. **Domain Allocation:** Validate point investment in adjacent nodes from domain centers outward.
3. **Perk & Bonus Application:** Apply stat modifiers (Health, Mana, Cap, Resistance, Spell Bouns) directly to `Player` stats during combat and stat recalculation.
4. **Presets:** Support preset swapping in Protection Zones.
