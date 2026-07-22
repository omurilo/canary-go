# Migration Spec: Offline Training Totem Refinement (Treino Offline por Totem)

## 1. Overview
Offline Training allows players to train skills (Sword, Axe, Club, Distance, Magic Level) up to 12 hours while logged off. Per user requirement, offline training must NOT accumulate automatically on logout; it must activate **only** when the player explicitly clicks an Offline Training Statue/Totem in-game.

## 2. C++ Source References
* `src/creatures/players/player.cpp` (`Player::checkOfflineTraining`)
* `data/scripts/actions/other/offline_training.lua`

## 3. Go Target Packages
* `canary-go/internal/game/player.go`
* `canary-go/data/scripts/creaturescripts/player/offline_training.lua`

## 4. Database Schema
* Table: `players`
  * `offline_training_time` (INT - remaining train time in seconds, max 43200 = 12h)
  * `offline_training_skill` (INT - active skill ID set by totem interaction)

## 5. Functional Requirements
1. **Totem Click Requirement:** Clicking an Offline Training Statue sets `offline_training_skill` and kicks/logs out the player.
2. **Passive Logout Prevention:** Logging out via normal exit/logout menu without interacting with a totem sets `offline_training_skill = -1` (no skill points gained).
3. **Login Calculation:** Upon login, if `offline_training_skill >= 0`, calculate elapsed offline time (clamped to available `offline_training_time`), award skill advances, and update remaining time.
