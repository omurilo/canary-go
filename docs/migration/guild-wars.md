# Migration Spec: Guild Wars (Guerras de Guildas)

## 1. Overview
The Guild War system allows two rival guilds to declare formal in-game war with agreed frag limits, fee stakes, duration, and end conditions. While at war, guild members render war shields (Green/Red emblem) and frags do not trigger Unjustified Kills (red/black skull).

## 2. C++ Source References
* `src/enums/guild_war.hpp`
* `src/server/network/protocol/protocolgame.cpp`

## 3. Go Target Packages
* `canary-go/internal/game/guild_war.go`
* `canary-go/internal/protocol/game_encode.go` (Guild emblem rendering)

## 4. Database Schema
* Table: `guild_wars` (`id`, `guild1`, `guild2`, `name1`, `name2`, `status`, `frag_limit`, `gold_fee`)
* Table: `guild_war_kills` (`war_id`, `killer`, `target`, `time`)

## 5. Functional Requirements
1. **War Declaration & Acceptance:** Support declaring war, setting frag limits, and accepting war invitations.
2. **War Emblem Rendering:** Broadcast war emblem icons over player characters (Green = Allied Guild, Red = Enemy War Guild).
3. **Frag Counting & Victory Condition:** End war automatically when a guild reaches the agreed frag limit or time expires, distributing the gold pool to the winner.
