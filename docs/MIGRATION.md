# Levantamento de Migração C++ → Go (Canary Server)

## Context

Você está migrando um servidor MMORPG Tibia (Canary) de C++ para Go. O C++ original possui ~174k LOC (486 arquivos) enquanto o Go possui ~26k LOC (206 arquivos). O servidor Go já implementa um "vertical slice" funcional - jogadores podem logar, andar, combater, usar NPCs, inventário e party - mas falta a maioria dos sistemas de progressão e features avançadas.

Este documento mapeia **o que ainda precisa ser migrado**, organizado por prioridade e impacto.

**Última atualização:** 2026-07-25

---

## Status Atual: O Que Já Funciona

**✅ Sistemas Completos:**
- Rede/Protocolo (RSA, XTEA, framing)
- Login e autenticação
- OTBM map loading (1.94M tiles)
- Catálogo de itens (appearances.dat + items.xml)
- Movimento (walk, turn, autowalk, floor changes, stairs, teleports)
- Inventário básico (slots, capacity, weight)
- Containers (open/close, add/remove items)
- Dinheiro e banco (coins, transfer)
- NPC dialogue, shop (buy/sell), travel, citizen
- Combate básico (melee + spell damage, death/respawn)
- Monstros básicos (spawn, aggro, pathfind, loot, xp)
- Party (invite, join, shields, shared exp toggle)
- Spells básicas (instant cast + damage)

**🟡 Sistemas Parciais (funcionam mas incompletos):**
- Player stats (falta skill tries, varStats, wheel bonuses)
- Conditions (regeneration/food ok, falta DoT, haste/paralyze speed, icons)
- Combat (falta PvP rules, damage split, elements/absorb, crit/leech, monster spells)
- Vocations (loaded, mas fórmulas per-vocation não aplicadas)
- Events (Action/MoveEvent/TalkAction ok, falta CreatureEvent/GlobalEvent/Zone)
- Persistence (player core + items salvos, falta skills, maglevel, conditions, storages, depot/inbox)
- Lua API (core functions, **414 stubs restantes**)

---

## O Que Falta: Matriz de Migração

### **PRIORIDADE A: Core Loop (maior impacto gameplay)**

#### A1. Conditions Engine Completo (~2,500 LOC C++)
**C++:** `src/creatures/combat/condition*.{cpp,hpp}`  
**Go:** `internal/game/conditions.go` (parcial)  
**Gap:**
- DoT (poison, fire, energy, bleeding, cursed) com ticks
- Haste/Paralyze **speed modifications** (precisa `0x8F sendChangeSpeed` + `Player.SpeedBonus`)
- Attribute conditions (+ skills, - stats)
- Drunk, light, outfit (invisibility), muted
- Status icons (`0xA2 sendIcons`) - veneno, haste, fome, PZ, etc.
- Condition persistence (save/load no DB)

**Fórmula Haste:** `ConditionSpeed` → `speed = mina*(var-40)+minb` com expiry

#### A2. Combat Completeness (~8,000 LOC C++)
**C++:** `src/creatures/combat/`  
**Go:** `internal/game/combat_engine.go`, `internal/game/combat/`  
**Gap:**
- Elementos e absorção (physical, fire, energy, earth, ice, holy, death)
- Defense/armor calculations completos
- Block chance (shield skill)
- Damage split entre múltiplos atacantes
- Critical hits e life/mana leech
- PvP rules (skull system, protection zone, level range)
- Monster spells e abilities (área, summon, heal, buff)
- Threat/aggro system
- Experience split em party

#### A3. Persistence Completa (~3,000 LOC C++)
**C++:** `src/io/player*.cpp`, `src/database/`  
**Go:** `internal/db/player.go`, `internal/io/`  
**Gap:**
- Skills + tries (sword, club, axe, distance, shield, fish, maglevel)
- Mana spent (para maglevel)
- Conditions ativas
- Player storages (quest flags, key-value)
- Spells aprendidas
- Blessings
- Stamina
- Town selecionada
- Depot items
- Inbox items  
- Stash (item storage system moderno)
- Tabelas DB: `player_storage`, `player_spells`, `player_depotitems`, `player_inboxitems`

#### A4. Status Icons & UI Feedback
**C++:** `src/server/network/protocol/protocolgame.cpp` (sendIcons)  
**Go:** `internal/protocol/` (falta `0xA2`)  
**Gap:**
- Packet 0xA2 com bitmask de icons
- Icons: poison, burn, energy, bleeding, haste, paralyze, swords (party), drunk, magic shield, etc.

---

### **PRIORIDADE B: Sistemas de Progressão (cada = model + DB + protocol + Lua)**

Cada subsistema aqui é uma feature completa que jogadores esperam. C++ possui **~40k LOC** destes sistemas.

#### B1. Mounts & Outfits (~2,000 LOC C++)
**C++:** `src/creatures/appearance/`, `src/game/movement/`  
**Go:** `internal/mounts/` (básico), falta outfit management  
**Gap:**
- Outfit storage e unlock system
- Mount storage e unlock system
- AddOn system (1, 2, 3)
- Protocol: change outfit packet
- Lua: `player:addOutfit()`, `player:hasMount()`, etc.
- DB: `player_outfits`, `player_mounts`

#### B2. Blessings (~800 LOC C++)
**C++:** `src/creatures/players/player.cpp` (blessings)  
**Go:** não implementado  
**Gap:**
- 8 blessings (twist of fate, etc.)
- Death penalty reduction
- Item protection
- Blessing buy em NPCs
- DB: bitmask em `players.blessings`

#### B3. Bestiary & Bosstiary (~1,600 LOC C++)
**C++:** `src/io/iobestiary.cpp`, `src/io/iobosstiary.cpp`  
**Go:** `internal/bestiary/`, `internal/bosstiary/` (stubs)  
**Gap:**
- Kill tracking por creature
- Unlock tiers (1→2→3)
- Charm points
- Bestiary window (protocol packets)
- Bosstiary tracking
- DB: `player_bestiary`, `player_bosstiary`

#### B4. Charms (~800 LOC C++)
**C++:** `src/creatures/players/player.cpp` (charms)  
**Go:** `internal/charms/` (básico)  
**Gap:**
- Charm buy/equip system
- Charm effects (dodge, zap, wound, etc.)
- Charm application em combat
- Lua bindings completos

#### B5. Prey System (~1,200 LOC C++)
**C++:** `src/creatures/players/components/player_prey.cpp`  
**Go:** `internal/game/prey.go` (stub?)  
**Gap:**
- 3 prey slots
- Reroll system (free + paid)
- Bonus types (damage, defense, exp, loot)
- Time tracking
- Window packets
- DB: `player_prey`

#### B6. Task Hunting (~1,000 LOC C++)
**C++:** `src/creatures/players/components/player_taskhunt.cpp`  
**Go:** `internal/game/task_hunter.go` (stub?)  
**Gap:**
- Task slots (free + premium)
- Boss tasks
- Task tracking
- Rewards
- DB: `player_taskhunt`

#### B7. Achievements & Titles (~1,500 LOC C++)
**C++:** `src/creatures/players/achievement/`, `src/creatures/players/cyclopedia/`  
**Go:** `internal/game/achievement.go`, `internal/db/achievements.go`  
**Status:** Model + DB + Lua bindings implementados. **Protocolo pendente (cyclopedia packets).**
- ✅ Achievement registry (ID/name/description/secret/points)
- ✅ Player unlock tracking (map[id]timestamp)
- ✅ DB: `player_achievements` + `player_titles` (Load/Save)
- ✅ Lua bindings: registerAchievement, getAchievementInfo, hasAchievement, addAchievement, getTitles, etc.
- ❌ Protocolo: cyclopedia achievement packet (0x61)
- ❌ Protocolo: SendCyopediaCharacterAchievements

#### B8. Wheel of Destiny (~6,074 LOC C++)
**C++:** `src/creatures/players/wheel/`  
**Go:** `internal/game/wheel/` (stub?)  
**Gap:**
- Skill tree completo
- Point allocation
- Stat bonuses (varStats)
- Special abilities
- Respec system
- Window packets
- DB: `player_wheeloffortune`

#### B9. Imbuements (~1,645 LOC C++)
**C++:** `src/creatures/players/imbuements/`  
**Go:** não implementado  
**Gap:**
- Imbuement slots em items
- Shrine system
- Material consumption
- Imbuement effects (crit, leech, elements)
- Duration tracking
- DB: item attributes

#### B10. Forge System (~800 LOC C++)
**C++:** Parte de `src/creatures/players/`  
**Go:** `internal/game/forge.go` (stub?)  
**Gap:**
- Item fusion
- Tier system
- Dust/core materials
- Success chances
- Window packets

#### B11. Market System (~650 LOC C++)
**C++:** `src/io/iomarket.cpp`  
**Go:** `internal/game/market.go`, `internal/db/market.go`  
**Status:** Model + DB implementados. **Protocolo + Lua não implementados.**
- ✅ MarketOffer model (buy/sell)
- ✅ In-memory Market cache (by ID, item, player)
- ✅ DB CRUD: Create/Remove/Get/LoadMarketOffers, `market_offers` table
- ❌ Protocolo: market browse (0xEF), create offer (0xF0), cancel (0xF1), buy (0xF2)
- ❌ Lua: openMarket é stub que chama interface inexistente
- ❌ Item delivery via inbox

#### B12. Houses (~1,689 LOC C++)
**C++:** `src/map/house/`  
**Go:** `internal/game/house.go`, `internal/db/houses.go`  
**Status:** Model + DB implementados. **Protocolo + Lua não implementados.**
- ✅ House model (ID, name, owner, rent, beds, size, access lists)
- ✅ World integration (register, lookup by ID/player)
- ✅ DB: LoadHouses, SaveHouseOwner, SaveAccessList
- ❌ Protocolo: house window (0x6F), edit door (0x70), rent/buy
- ❌ Lua bindings: nenhum
- ❌ Rent system (payHouses cron)

#### B13. Guilds (~1,683 LOC C++ em grouping)
**C++:** `src/creatures/players/grouping/guild.cpp`  
**Go:** não implementado  
**Gap:**
- Guild creation
- Ranks system
- Invites/kicks
- Guild hall
- Motd
- DB: `guilds`, `guild_membership`, `guild_ranks`, `guild_invites`

#### B14. VIP System (~500 LOC C++)
**C++:** Parte de `src/creatures/players/`  
**Go:** não implementado  
**Gap:**
- VIP list (add/remove)
- Online status
- VIP groups
- Notificações
- DB: `player_viplist`

#### B15. Familiars (~300 LOC C++)
**C++:** `src/creatures/players/grouping/familiars.cpp`  
**Go:** `internal/game/familiar.go`, `internal/db/familiars.go`  
**Status:** Model + DB + Lua implementados. **Protocolo pendente (familiar window).**
- ✅ Familiar model (lookType, name, premium, type)
- ✅ Player unlock/remove/has/set active familiar
- ✅ DB: `player_familiars` table (Load/Save)
- ✅ Lua bindings: addFamiliar, removeFamiliar, hasFamiliar, setFamiliarLooktype, getFamiliarLooktype
- ❌ Protocolo: familiar window packet

#### B17. Hazard System (LOC desconhecido)
**C++:** Parte de `src/game/`  
**Go:** não implementado  
**Gap:** Sistema completo

#### B18. Concoctions (LOC desconhecido)
**C++:** Parte de `src/items/`  
**Go:** não implementado  
**Gap:** Sistema completo

#### B19. Store & Tibia Coins (LOC desconhecido)
**C++:** Parte de `src/server/network/protocol/`  
**Go:** não implementado  
**Gap:**
- Store window
- Coin purchase/spend
- Offers catalog
- Premium time

#### B20. Reward System (LOC desconhecido)
**C++:** Parte de `src/items/containers/`  
**Go:** não implementado  
**Gap:**
- Reward chest
- Boss loot system
- Auto-loot configuration

---

### **PRIORIDADE C: Infraestrutura & Engine**

#### C1. Container Hierarchy (~2,553 LOC C++)
**C++:** `src/items/containers/`  
**Go:** `internal/game/player_containers.go` (básico)  
**Gap:**
- Depot (per-town storage)
- Inbox (market/mail delivery)
- Mailbox (send mail)
- Rewards container
- Store inbox
- Container pagination (depot boxes)
- DB: depot/inbox como item tree

#### C2. Decay System (~500 LOC C++)
**C++:** `src/items/decay/`  
**Go:** não implementado  
**Gap:**
- Item decay tracking
- Transform chains (corpse→bones→nothing)
- Time-based transformations

#### C3. Weapons System (~800 LOC C++)
**C++:** `src/items/weapons/`  
**Go:** não implementado  
**Gap:**
- Weapon types (distance, melee, wand, ammunition)
- Attack calculations por weapon type
- Weapon attributes (elements, effects)

#### C4. Advanced Scheduling (~3,877 LOC C++)
**C++:** `src/game/scheduling/`  
**Go:** `internal/game/dispatcher.go` (básico)  
**Gap:**
- WDRR (Weighted Deficit Round Robin) scheduler
- Monster compute service
- Save manager (auto-save)
- Telemetry
- Budget control

#### C5. Zones System (~950 LOC C++)
**C++:** `src/game/zones/`  
**Go:** não implementado  
**Gap:**
- Zone definitions
- Zone types (PvP, no-logout, protection)
- Zone events
- Zone Lua integration

#### C6. Modal Windows (~300 LOC C++)
**C++:** `src/game/modal_window/`  
**Go:** não implementado  
**Gap:**
- Modal window packets
- Button handling
- Choice callbacks

#### C7. Bank System (~430 LOC C++)
**C++:** `src/game/bank/`  
**Go:** `internal/game/player_money.go` (banco básico via Lua)  
**Gap:**
- Sistema completo em Go (não só Lua)
- History tracking

#### C8. Account Management (~950 LOC C++)
**C++:** `src/account/`  
**Go:** `internal/db/account.go` (básico)  
**Gap:**
- Premium account tracking
- Account coins
- Creation date
- Recovery key

#### C9. KV Store (~928 LOC C++)
**C++:** `src/kv/`  
**Go:** não implementado  
**Gap:** Sistema key-value global (não player storage)

---

### **PRIORIDADE D: Eventos & Lua (long tail)**

#### D1. CreatureEvent System (parte dos ~42k LOC Lua C++)
**C++:** `src/lua/callbacks/`, `src/lua/creature/`  
**Go:** `internal/events/` (stub)  
**Gap:**
- onLogin, onLogout
- onThink
- onDeath, onKill
- onAdvance
- onModalWindow
- Registration system
- ~560 load warnings no boot

#### D2. GlobalEvent System
**C++:** `src/lua/global/`  
**Go:** não implementado  
**Gap:**
- Startup events
- Server save
- Time-based events (hourly, daily)
- Record events

#### D3. Zone Events (integrado com C5)
**Go Gap:** Zone-based triggers

#### D4. Lua API Completo (~1,300 funções, 414 stubs restantes)
**C++:** `src/lua/functions/` (~37k LOC)  
**Go:** `internal/luaengine/` (parcial)  
**Gap (exemplos dos 414 stubs):**
- Player: varStats, wheel points, skull system, guild methods, VIP, bestiary, prey, task hunt, blessings, imbuements, forge, etc.
- Game: place/remove creatures, forceRaid, startRaid
- Item: decay, item attributes avançados
- Monster: spell casting, summons
- Combat: área effects, conditions complexas

---

### **PRIORIDADE E: Features Avançadas**

#### E1. Raids System
**C++:** scripts de raids  
**Go:** não implementado  
**Gap:**
- Raid scheduler
- Monster waves
- Boss spawns
- GlobalEvent integration

#### E2. Quests System (só scripts Lua, mas...)
**Gap:**
- Quest log tracking
- Quest line UI
- Mission tracking
- DB: player quest states

#### E3. World Changes
**Gap:**
- Timed content (Rapid Respawn)
- Event-based spawns

#### E4. Livestream System (~1,900 LOC C++)
**C++:** `src/creatures/players/livestream/`  
**Go:** não implementado  
**Gap:** Sistema completo de streaming

#### E5. Webhooks (~500 LOC C++)
**C++:** `src/server/network/webhook/`  
**Go:** não implementado  
**Gap:** Discord/external integrations

#### E6. Protocol Extensions
**Gap:**
- Protocol Status (server info)
- Protocol Profile (character profile)

---

## Resumo Quantitativo

| Categoria | C++ LOC | Go Status | Gap Estimado |
|-----------|---------|-----------|--------------|
| **Core Loop (A)** | ~13,500 | 30% | ~9,500 LOC |
| **Progressão (B)** | ~20,000 | 5% | ~19,000 LOC |
| **Infraestrutura (C)** | ~10,000 | 20% | ~8,000 LOC |
| **Eventos/Lua (D)** | ~42,000 | 20% | ~33,000 LOC |
| **Features Avançadas (E)** | ~5,000 | 0% | ~5,000 LOC |
| **TOTAL ESTIMADO** | **~90,500** | **17%** | **~75,000 LOC** |

(Restante do C++ são utils, libs, parsing, protobuf - já cobertos ou não necessários)

---

## Ordem Sugerida de Implementação

### Fase 1: Fechar o Core Loop (3-4 semanas) ✅ EM ANDAMENTO
1. **A1**: Conditions + haste/paralyze speed + icons
2. **A4**: Status icons packet (depende de A1)
3. **A3**: Persistence de skills/maglevel/conditions/storages
4. **A2**: Combat completo (elements, PvP, monster spells)

**Resultado:** Jogadores têm progressão completa que persiste, DoT funciona, combate parece "real"

### Fase 2: Containers & Storage (1-2 semanas)
5. **C1**: Depot/Inbox/Mailbox
6. **A3 (parte 2)**: DB persistence para depot/inbox

**Resultado:** Jogadores podem guardar itens, receber do market

### Fase 3: Progressão Essencial (4-6 semanas, 1 sistema por vez)
7. **B2**: Blessings (rápido, alto impacto)
8. **B1**: Mounts & Outfits (visual appeal)
9. **B14**: VIP System (social)
10. **B3**: Bestiary & Bosstiary
11. **B4**: Charms
12. **B5**: Prey
13. **B6**: Task Hunting

**Resultado:** Loop de progressão PvE completo

### Fase 4: Economia & Social (3-4 semanas)
14. **B12**: Houses
15. **B13**: Guilds
16. **B11**: Market

**Resultado:** Economia de jogadores funciona

### Fase 5: Sistemas Avançados (6-8 semanas)
17. **B8**: Wheel of Destiny (complexo)
18. **B9**: Imbuements
19. **B10**: Forge
20. **B7**: Achievements & Titles
21. Outros sistemas B conforme demanda

### Fase 6: Polish & Features (ongoing)
22. **C2-C9**: Sistemas de infraestrutura restantes
23. **D1-D4**: CreatureEvent, GlobalEvent, Lua API completo
24. **E1-E6**: Features avançadas

---

## Arquivos Críticos para Referência

Ao migrar cada sistema, consulte:

**C++ Reference:**
- `src/creatures/players/player.{cpp,hpp}` - modelo Player core
- `src/server/network/protocol/protocolgame.cpp` - todos os packets (0x00-0xFF)
- `src/lua/functions/creatures/player/player_functions.cpp` - 5,600 linhas de Lua API
- `src/game/game.cpp` - game loop e world rules

**Go Target:**
- `internal/game/player.go` - modelo Player
- `internal/protocol/game.go` - packet handlers
- `internal/luaengine/player.go` - Lua bindings
- `internal/game/world.go` - game engine

---

## Métricas de Progresso

- **LOC migradas:** ~27k / ~100k (27%)
- **Sistemas funcionais:** 15 / ~50 (30%) — inalterado (os sistemas B7/B11/B12/B15 só têm model+DB, protocolo pendente)
- **Lua stubs restantes:** ~130 / ~1300 (90% done, 10% critical)
- **Packets implementados:** 31 / ~60 (52%)
- **DB tables migradas:** 12 / ~25 (48%) — +4: player_achievements, player_titles, player_familiars, market_offers, houses

---

## Notas Importantes

1. **Cada sistema B requer 4 componentes:**
   - Model em `internal/game/` ou `internal/`
   - DB table + queries em `internal/db/`
   - Protocol packets em `internal/protocol/`
   - Lua bindings em `internal/luaengine/`

2. **Validação:** Cada feature deve ser testada no cliente oficial BattlEye 13.x

3. **HANDOFF.md existente** documenta invariantes críticos (metatable contract, wire format, etc.)

4. **414 Lua stubs** mapeiam exatamente para os sistemas B-E acima

5. **~560 load warnings** no boot são por falta de CreatureEvent/GlobalEvent/Zone registration (D1-D3)
