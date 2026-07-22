# Migration Spec: In-Game Store & Tibia Coins (Store e Tibia Coins)

## 1. Overview
The In-Game Store allows purchasing outfits, mounts, XP boosts, blessings, name changes, sex changes, and extra prey wildcards using transferrable/transferable Tibia Coins.

## 2. C++ Source References
* `src/server/network/protocol/protocolgame.cpp` (Store Opcodes `0xFB` - `0xFE`)

## 3. Go Target Packages
* `canary-go/internal/game/store.go`
* `canary-go/internal/protocol/game_actions.go`

## 4. Database Schema
* Table: `accounts` (`coins`, `coins_transferable`)
* Table: `store_history` (`account_id`, `item_name`, `coin_cost`, `time`)

## 5. Protocol Opcodes
* Client → Server: `0xFB` (Open Store), `0xFC` (Buy Store Item)
* Server → Client: `0xFD` (Store Category/Offers Payload), `0xFE` (Store Purchase Transaction Result)

## 6. Functional Requirements
1. **Coin Deduction:** Validate and deduct Tibia Coins atomically upon purchase.
2. **Instant Delivery:** Grant outfits, mounts, XP boosts, or items instantly upon confirmation.
3. **Transaction Logging:** Store purchase audit logs in `store_history`.
