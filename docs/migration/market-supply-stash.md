# Migration Spec: Market & Supply Stash (Mercado e Stash de Suprimentos)

## 1. Overview
The Market system enables asynchronous buying/selling of tradeable items between players with gold escrow. The Supply Stash is an infinite stackable item storage accessible at Depots for stackable materials, pot ingredients, and creature products.

## 2. C++ Source References
* `src/items/containers/stash/stash.cpp`
* `src/server/network/protocol/protocolgame.cpp` (Market Opcodes `0xF5` / `0xF6`)

## 3. Go Target Packages
* `canary-go/internal/game/market.go`
* `canary-go/internal/game/stash.go`
* `canary-go/internal/protocol/game_actions.go`

## 4. Database Schema
* Table: `market_offers` (`id`, `player_id`, `type`, `itemtype`, `amount`, `price`, `created`)
* Table: `player_stash` (`player_id`, `item_id`, `count`)

## 5. Protocol Opcodes
* Client → Server: `0xF5` (Market Leave/Browse/Offer), `0x28` (Stash Deposit/Withdraw)
* Server → Client: `0xF6` (Market Offer List & History Data)

## 6. Functional Requirements
1. **Market Offer Creation:** Escrow gold for buy offers and escrow items for sell offers.
2. **Offer Matching & Instant Fill:** Automatically match compatible buy/sell offers and deposit gold/items directly into player bank/depot.
3. **Stash Deposit/Withdraw:** Support moving stackable items directly into/out of the player's Stash from Depot boxes or inventory.
