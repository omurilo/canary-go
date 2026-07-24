# Migration Gap Analysis — C++ → Go

_Last updated: 2026-07-24_

Levantamento do que ainda falta para migrar 100% do servidor Canary (C++, `../src`)
para o port Go (`canary-go/internal` + datapack em `canary-go/data`).

**Estado geral:** o *core loop* está jogável — login, movimento, combate básico,
chat, containers, itens, loja, morte/exp, e os sistemas de progressão já
portados (bestiary, bosstiary, charms, prey, task hunting, wheel parcial, forge
parcial). Falta, grosso modo, **~45–55% da largura de features**: profundidade
de combate (crit/leech/reflect/resistências de monstro), casas, market,
imbuements, boa parte dos opcodes de protocolo, páginas da cyclopedia,
achievements, hirelings, reward chest, e a maioria das classes Lua.

> ⚠️ **Mais urgente:** a persistência tem **perda silenciosa de dados** — depot,
> inbox e reward chest não são salvos e somem no relog.

Método: auditoria de progressão e persistência feita por leitura completa do
código; os demais clusters foram fundamentados por inspeção direta (contagem de
opcodes + greps). Números são aproximados (contagem de handlers/bindings), não
exatos linha-a-linha.

---

## 1. Combate & Spells — parcial (~50%)

| Subsistema | Status | Falta | Esforço |
|---|---|---|---|
| Pipeline de dano (melee/wand/distance, block armor/shield, resistência elemental) | ✅ Feito | aplicado em `combat/combat.go` | — |
| Condições (poison/fire/energy/haste/paralyze/regen…) | ✅ Feito | — | S |
| Spells (instant/rune, ~200 scripts do datapack) | 🟡 Parcial | casting funciona via `spell.go`; falta cooldown de grupo, gating fino por vocação, conjure | M |
| Crit chance/damage | ❌ Faltando | não há pipeline de crit (por isso Low/Savage charm são aproximados) | M |
| Life/Mana leech | ❌ Faltando | só existe no wheel, não no combate base | M |
| Reflect / absorb de monstro | ❌ Faltando | `reflect`/`elementMap` de monstro = 0 no engine | M |
| PvP (skulls, war, frags) | ❌ Faltando | sem sistema de skull/war | L |

## 2. Criaturas — parcial (~55%)

| Subsistema | Status | Falta | Esforço |
|---|---|---|---|
| Players (skills, exp/level, morte/penalidade, regen, vocação básica) | ✅ Feito | `attackSpeed` fixo (TODO vocação) | S–M |
| Monstros: AI alvo, ataques/spells, loot básico | 🟡 Parcial | combate por string-match simplificado | M |
| Monstro: summon, flee, resistências elementais | ❌ Faltando | `summon`/`flee`/`resistance` = 0 no engine | L |
| NPCs (diálogo, shops, travel) | ✅ Feito (via Lua) | — | — |
| Fiendish/Influenced | ✅ Feito (light) | — | — |

## 3. Itens & Economia — parcial (~45%)

| Subsistema | Status | Falta | Esforço |
|---|---|---|---|
| Itens core, containers, decay, armas/armadura | ✅ Feito | — | — |
| In-game Store + coins | ✅ Feito | — | — |
| Forge (Exaltation) | 🟡 Parcial | fusion/dust básicos; falta transfer/convert completos | M |
| Imbuements | ❌ Faltando | `Imbuement` é mockClass | L |
| Market | ❌ Faltando | só stub | L |
| Trade entre players | 🟡 Parcial/Faltando | verificar | M |

## 4. Mapa / Mundo / Protocolo — parcial (~40%)

| Subsistema | Status | Falta | Esforço |
|---|---|---|---|
| Mapa OTBM, tiles/flags, pathfinding A* | ✅ Feito (básico) | — | — |
| Movimento, teleports, stairs, MoveEvents | ✅ Feito | — | — |
| Spawns de monstro | 🟡 Parcial | respawn básico | M |
| Casas (rent/auction/beds/doors/guests) | ❌ Faltando | sem engine (`House` mockClass) | XL |
| Raids / invasions | ❌ Faltando | mínimo | L |
| Protocolo | 🟡 Parcial | **27/120 recv, 32/187 send** — core coberto; faltam ~90 handlers auxiliares | XL |

## 5. Progressão / Cyclopedia — parcial _(auditoria completa)_

| Sistema | Status | Falta |
|---|---|---|
| Bestiary / Bosstiary | ✅ Feito | detalhes cosméticos (armor/mitigation/resist = 0) |
| Charms | 🟡 Parcial | Scavenge/Gut/Fatal/VoidInversion sem subsistema; Low/Savage aproximados |
| Prey / Task Hunting | 🟡 Parcial | grid fixo, dificuldade não derivada de stars |
| Wheel of Destiny | 🟡 Parcial | gems/vessels, revelation, spells do wheel inertes |
| Quest log (0xF0/0xF1) | ❌ Faltando | storages funcionam, mas log vazio no cliente |
| Achievements / Badges / Titles | ❌ Faltando | stubs no-op |
| Cyclopedia (11 páginas de char info) | 🟡 Parcial | só Inspection (tipo 9); faltam stats/deaths/kills/etc |
| Hirelings / Familiars / Reward chest / Daily reward | ❌ Faltando | sem engine |
| Mounts / Outfits | ✅ Feito (light) | outfits sem gating de posse |

Nota de wiring (verificado em `internal/game/combat_engine.go`): ao matar um
monstro o engine credita bestiary, bosstiary (com entry-changed), task-hunter e
aplica bônus de prey (XP/loot/dano/defesa) e procs de charm. O lado de "kill" é
sólido; os gaps acima são majoritariamente senders de UI/protocolo e subsistemas
auxiliares.

## 6. Persistência / Conta / Lua API — parcial _(auditoria completa)_

| Subsistema | Status | Falta |
|---|---|---|
| Load/Save player (~55%) | 🟡 Parcial | **NÃO salva: depot, inbox, reward chest, stash, spells aprendidos, kills/skull, condições persistentes** → perda silenciosa no relog |
| Contas & login | 🟡 Parcial | sem 2FA, session-key, `coins_tournament`, VIP list |
| Guilds | 🟡 Parcial | só leitura; sem banco/ranks/guild wars |
| Parties | ✅ Feito | — |
| Classes Lua (~55–60%) | 🟡 Parcial | Player/Creature/Game/Monster/Spell OK; mockClass: House, Weapon, Vocation, Zone, ModalWindow, Guild, Imbuement, KV… |
| EventCallback (3/37) | ❌ Faltando | só onLogin/onLook/onMoveItem; 34 hooks no-op |
| Action/MoveEvent/TalkAction/CreatureEvent/GlobalEvent | ✅ Feito | CreatureEvent só login/death |

### Fatos que sustentam peso

- `SavePlayerItems` grava só `player_items` (slots 1–11). C++ grava também
  `player_depotitems`, `player_inboxitems`, `player_rewards`. **Depot, inbox e
  reward chest não persistem.**
- VIP list (`account_viplist`/grupos): sem equivalente Go.
- `coins_tournament` (CoinType::Tournament): não modelado (só `coins` e
  `coins_transferable`).
- `mockClass` em `internal/luaengine/api.go`: Weapon, Achievement, ItemTier,
  Spawns, BedItem, ItemClassification, Teleport, EventCallback, Vocation,
  GemAtelier, Guild, Group, House, Zone, Hazard, ZoneEvent, Webhook — retornam
  userdata inerte (não crasham, mas não fazem nada).

---

## Prioridades sugeridas

### 🔴 Crítico (corrige perda de dados / bugs)
1. Persistir **depot / inbox / reward chest / stash / spells aprendidos** (hoje somem no relog).
2. `coins_tournament` e VIP list.

### 🟠 Alto impacto de gameplay
3. **Crit + life/mana leech + resistências de monstro** no combate (destrava Low/Savage charm e balanceamento).
4. **Cobertura de protocolo** — priorizar quest log (0xF0/0xF1), cyclopedia char pages, market.
5. **EventCallback** (34 hooks) — muitos scripts do datapack dependem deles.

### 🟡 Sistemas grandes ainda ausentes
6. **Casas**, **Market**, **Imbuements**, **Hirelings**, **Reward/Daily reward**, **Achievements/Titles/Badges**.
7. Completar **Wheel** (gems/vessels/spells), **Guild** (write path + wars), classes Lua mockadas (Weapon/Vocation/ModalWindow…).

**Estimativa honesta:** core jogável, mas ~45–55% da superfície total do C++. As
maiores empreitadas rumo a "100%" são casas, market, imbuements, cobertura de
protocolo e a superfície Lua.

---

_Clusters "Progressão" e "Persistência/Conta/Lua" vieram de auditoria completa
(leitura de código). Combate, Criaturas, Itens e Mapa/Protocolo foram
fundamentados por inspeção direta desta data (contagem de opcodes + greps de
market/imbue/house/crit/leech/summon/flee/spawn). Para detalhamento
linha-a-linha de qualquer cluster, rodar auditoria dedicada por subsistema._
