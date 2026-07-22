# Migration Spec: Imbuement System (Sistema de Imbuimentos)

## 1. Overview
The Imbuement system allows applying temporary elemental damage, vampirism (Life/Mana Leech), critical hit chance, or skill bonuses to weapons, armors, and shields at an Imbuing Shrine using creature products.

## 2. C++ Source References
* `src/items/imbuement/imbuement.hpp`
* `src/items/imbuement/imbuement.cpp`
* `src/server/network/protocol/protocolgame.cpp` (Opcodes `0xD3` Imbuing Window)

## 3. Go Target Packages
* `canary-go/internal/game/imbuement.go`
* `canary-go/internal/game/item_attr.go`
* `canary-go/internal/game/combat_engine.go` (Life/Mana Leech & Critical Hit handling)

## 4. Database Schema
* Item custom attributes (stored serialized in item text attributes)
  * `imbuement_type`, `imbuement_level`, `time_left`

## 5. Protocol Opcodes
* Client → Server: `0xD3` (Apply Imbuement / Clear Imbuement)
* Server → Client: `0xD2` (Open Imbuing Shrine Window)

## 6. Functional Requirements
1. **Shrine Interaction:** Opening an Imbuing Shrine displays available slots (1-3) based on item category.
2. **Combat Integration:** `combat_engine.go` must apply Life Leech % and Mana Leech % to player health/mana recovery during physical & spell attacks.
3. **Timer Decay:** Decrement imbuement duration only while the item is equipped in an active slot during combat.
