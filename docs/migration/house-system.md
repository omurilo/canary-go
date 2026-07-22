# Migration Spec: House System (Sistema de Casas)

## 1. Overview
Houses are purchasable player residences. The House system handles door permissions (`aleta sio`, `aleta som`, `aleta grav`), house auctions (bidding via website/AAC), monthly rent collection, item transfer upon owner loss, and guest kicking (`alana sio`).

## 2. C++ Source References
* `src/map/house/house.hpp`
* `src/map/house/house.cpp`

## 3. Go Target Packages
* `canary-go/internal/game/house.go`
* `canary-go/internal/game/map.go`

## 4. Database Schema
* Table: `houses` (`id`, `owner`, `paid`, `price`, `rent`, `beds`)
* Table: `house_lists` (`house_id`, `list_id`, `text`)

## 5. Functional Requirements
1. **Door Access Control:** Check owner, sub-owner (`aleta som`), and guest list (`aleta sio`) when opening house doors.
2. **Guest Eviction (`alana sio`):** Teleport target player out of the house to the front door tile.
3. **Rent Collection & Eviction:** Periodically deduct house rent from bank account; if balance is insufficient, clear ownership and move house items to depot.
