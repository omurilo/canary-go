# Canary Go Migration Specs

This directory contains individual technical specifications for migrating C++ Canary features (`src/`) into the Go server implementation (`canary-go/`).

## Specifications Index

### 1. Player Progression Systems (High Priority)
* [Wheel of Destiny (Roda do Destino)](wheel-of-destiny.md)
* [Exaltation Forge (Forja de Exaltação)](exaltation-forge.md)
* [Bestiary & Charms (Bestiário e Charms)](bestiary-charms.md)
* [Prey & Task Hunter](prey-task-hunter.md)
* [Bosstiary & Boss Slot](bosstiary.md)
* [Imbuement System (Imbuimentos)](imbuement-system.md)
* [Offline Training Totem Refinement (Treino Offline por Totem)](offline-training.md)

### 2. Monsters, Bosses & Events
* [Fiendish & Influenced Monsters](fiendish-influenced-monsters.md)
* [Hazard System (Sistema de Hazard)](hazard-system.md)
* [Raids & Invasions Engine (Raids e Invasões)](raids-engine.md)
* [Advanced Boss AI (IA Avançada de Bosses)](advanced-boss-ai.md)

### 3. Economy, Houses & Social Systems
* [House System (Sistema de Casas)](house-system.md)
* [Market & Supply Stash (Mercado e Stash de Suprimentos)](market-supply-stash.md)
* [Guild Wars (Guerras de Guildas)](guild-wars.md)

### 4. Protocol & Client UI (Tibia 13.x)
* [In-Game Store & Tibia Coins](in-game-store.md)
* [Quick Loot / Auto-Loot Filtering (Coleta Rápida)](quick-loot.md)

### 5. Lua Engine Parity
* [Lua Engine Parity & Missing Bindings](lua-engine-parity.md)

---

## Migration Philosophy

* **Safety & Parity:** Go structures match the authoritative C++ state definitions and DB schema.
* **Non-Blocking Architecture:** High-throughput systems (AI, Combat, Spawns) leverage Go channels and goroutines cleanly.
* **Lua Integration:** All gameplay features exposed to content scripts provide `gopher-lua` bindings matching standard Canary syntax.
