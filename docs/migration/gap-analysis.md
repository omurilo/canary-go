# Migration Gap Analysis — C++ → Go

_Last updated: 2026-07-24 — auditoria completa (line-cited) dos 6 clusters._

Levantamento do que ainda falta para migrar 100% do servidor Canary (C++, `../src`)
para o port Go (`canary-go/internal` + datapack em `canary-go/data`).

**Estado geral:** o *core loop* está jogável — login, movimento, combate básico,
chat local, containers, itens, loja, morte/exp, e os sistemas de progressão já
portados (bestiary, bosstiary, charms, prey, task hunting, wheel parcial, forge
completo). Falta, grosso modo, **~45–55% da largura de features**: profundidade
de combate (crit/leech/reflect/absorb), casas, market, imbuements, trade, boa
parte dos opcodes de protocolo (canais, VIP, quest log, modal, cyclopedia),
achievements, hirelings, reward chest, e a maioria das classes Lua.

> ⚠️ **Mais urgente:** persistência com **perda silenciosa de dados** — depot,
> inbox e reward chest não são salvos e somem no relog.

---

## 🐞 Bugs concretos encontrados na auditoria (baixo esforço, corrigir já)

1. **`ConditionSpeedStruct.uniformRandom` sempre retorna `max`** (`internal/game/combat/condition.go:276-281`) — haste/paralyze (e os charms Adrenaline/Numb/Cripple) nunca randomizam; sempre aplicam efeito máximo.
2. **`fmt.Printf` de debug no hot path de spell** (`internal/game/spell_combat.go:139`) — vaza no stdout a cada cast.
3. **`combat_formulas.go` é um segundo caminho de fórmula melee/dist divergente** (`/50` vs `0.085*factor` do engine) e não é usado pela engine viva — risco de drift; deletar ou reconciliar.
4. **A\* pathfinding é stub** (`internal/game/pathfinding.go:26,37-38`) — "cai para perseguição direta", sem checagem de obstáculo → monstros e auto-walk andam contra paredes. (Alto impacto, não é bug trivial.)
5. **Toggle-mount hardcoda mount id 388** (`internal/protocol/outfit_handlers.go:201-214`) — ignora as mounts que o player possui.

---

## 1. Combate & Spells — parcial (~50%)

Ref: `src/creatures/combat/{combat,condition,spells}.cpp`. Go: `internal/game/combat_engine.go`, `internal/game/combat/*.go`, `spell_combat.go`, `internal/luaengine/{spell,combat,condition}.go`.

| Subsistema | Status | Falta | Esforço |
|---|---|---|---|
| Fórmulas de dano (melee/dist/wand/spell) | 🟡 Parcial | fórmulas existem; `combat_formulas.go` é caminho duplicado/divergente; tipo de combate de spell de monstro é inferido por string-match | M |
| Block (armor / defense / shielding) | 🟡 Parcial | subtração flat; falta roll probabilístico de shielding (`blockHit`), block-count/interval, imunidade, mitigation do wheel; armor de monstro sempre 0 | M |
| Resistência / absorb elemental | 🟡 Parcial | monstros aplicam via adapter, mas **players não têm absorb/resist** (adapter retorna 0) — gear/imbuement absorption ausentes | L |
| **Reflect** | ❌ Faltando | nada de reflect no Go (C++ `game.cpp:7891-8011`) | M |
| **Crit chance/damage** | ❌ Faltando | sem crit no pipeline (por isso Low/Savage charm aproximados) | M |
| **Life/Mana leech** | ❌ Faltando | só no wheel | S |
| **Secondary damage type** | ❌ Faltando | `CombatDamage` tem os campos mas `DoCombatHealth` só processa o primário | S |
| doCombatHealth/Mana + área | 🟡 Parcial | single/area/mana-drain/heal wired; falta manashield-redirect, pipeline completo (blessing/death-string/extension) | M |
| Condições DoT (poison/fire/energy/bleed/freeze/curse) | 🟡 Parcial | tick funciona via Lua `addDamage`; falta a curva de dano auto-gerada do C++ (`generateDamageList`) | M |
| Condições haste/paralyze | 🟡 Parcial | **bug: `uniformRandom` sempre max** | S |
| Condições regen/attributes | 🟡 Parcial | regen de comida é 1hp/1mp fixo, não por vocação | M |
| manashield / invisible / drunk / root / drowning | ❌ Faltando | enums existem, sem comportamento | L |
| Spells: cast checks (words/level/mana/soul/cooldown/vocação) | ✅ Feito | — | — |
| Spells: rune | 🟡 Parcial | `runeId/charges/allowFarUse` no-op — runas **não ligadas ao uso de item** | L |
| Spells: conjure | ❌ Faltando | 49 scripts, mas não criam item | M |
| Spells: efeitos agressivos no cast (pzLock/skull/needWeapon) | 🟡 Parcial | armazenados, não aplicados | M |
| Cobertura de scripts de spell | 🟡 Parcial | ~200 de 793 (~25%) | L |
| PvP: protection zone / secure mode | ✅ Feito | — | — |
| PvP: skulls (white/red/black/frag) | ❌ Faltando | só o campo `Skull`; sem atribuição/justificação | XL |
| PvP: world type enforcement | ❌ Faltando | `WorldType` guardado mas nunca consultado; PvP é `/2` fixo | M |
| PvP: party (dano/share/skull) | 🟡 Parcial | membership existe, sem integração de combate | L |
| PvP: guild war | ❌ Faltando | inexistente | XL |

## 2. Criaturas — parcial (~50%)

| Subsistema | Status | Falta | Esforço |
|---|---|---|---|
| **Monster spell system** | 🟡 Parcial | **damage-only**: ignora condition/area/wave/radius/script/`NeedTarget`, cadência de interval (roda todo tick), sons; tipo por substring | XL |
| Monster AI / targeting | 🟡 Parcial | só "player atacável mais próximo" 1s; sem estratégias, sem monster-vs-monster/summon, target limpo todo tick | L |
| Monster movement | 🟡 Parcial | A* 100 nós; sem keep-distance (ranged), sem flee, sem push | M |
| Monster defenses / self-heal | ❌ Faltando | `addDefense` no-op; sem healing/haste defensivo | L |
| Monster reflect / resist elemental | 🟡 Parcial | **resistências só no loader XML morto**; o loader Lua (real) não parseia `monster.elements` → % vazio; reflect ausente | M |
| Summons | ❌ Faltando | sem parsing/spawn/cap/lifecycle | L |
| Loot drop | 🟡 Parcial | funciona; falta rate/jitter/gut/charm-bonus/de-dup | S |
| Fleeing (runAwayHealth) | ❌ Faltando | flag parseada, nunca lida | M |
| Bestiary/bosstiary hooks | ✅ Feito | — | — |
| Fiendish / influenced | 🟡 Parcial | fiendish ok; sem influenced scheduler, sem soul-pit, roll difere do C++ | M |
| NPC diálogo/keyword | 🟡 Parcial | via Lua; vários hosts stub (`Say`/`TurnToCreature` vazios, `getShopItem` nil) | M |
| NPC shops | 🟡 Parcial | buy/sell ok; sem backpack/stack-aware, currency só gold | M |
| NPC travel | 🟡 Parcial | via Lua StdModule | S |
| NPC behaviours (walk/idle/yell) | ❌ Faltando | NPCs estáticos | M |
| Player skills & tries | 🟡 Parcial | base fixa, ignora multiplicadores de vocação; cap 150 hardcoded | M |
| Player magic level | 🟡 Parcial | ignora `manamultiplier` da vocação | S |
| Vocations | 🟡 Parcial | usa attackspeed/formula; ignora gain ticks/amounts; sem promoção | M |
| Leveling / experience | 🟡 Parcial | **level-up só refila, não cresce HP/mana/cap** (TODO); sem stamina/share/boost | L |
| **Death & penalty** | 🟡 Parcial | **só remove exp**; sem perda de skill/mag, sem drop de item, sem downgrade de stats, sem skull | L |
| Regeneration | 🟡 Parcial | 1hp/1mp fixo, ignora amounts da vocação; sem soul regen | M |
| Soul / Capacity / Stamina | 🟡/❌ | soul e cap parciais (sem crescimento por level); **stamina inexistente** | M |
| Mounts / Outfits | 🟡 Parcial | toggle hardcoda id 388; sem bônus de velocidade; addons zerados; sem gating de posse | M |
| Conditions (player) | ✅ Feito | deadlock-safe, modifiers wired | — |
| Storage / KV | 🟡 Parcial | storages em memória+DB; **sem KV store scoped** | M |

## 3. Itens & Economia — parcial (~45%)

| Subsistema | Status | Falta | Esforço |
|---|---|---|---|
| Item types / catalog | 🟡 Parcial | do `appearances.dat`, não `.otb`; sem mapa server↔client id | M |
| Item attribute TLV | 🟡 Parcial | decode bom; container-items/custom caem em raw-blob; imbuement-slot/mantra ignorados | M |
| Containers / stackables | ✅ Feito | — | — |
| Decay | 🟡 Parcial | **só itens de tile**; sem decay-to-nothing, sem inventário, sem resume no load | M |
| Item transformation (equip/de-equip) | ❌ Faltando | parseado, nunca aplicado | S |
| Weapon/armor/ammo stats | 🟡 Parcial | acessores ok; sem max-hit-chance/element no dano; consumo de ammo não confirmado | M |
| Inventory/equipment slots | 🟡 Parcial | **capacidade não gated** em equip/buy; sem conflito 2-mãos↔shield; bônus de gear stub 0 | M |
| Item conditions from gear | ❌ Faltando | sem bônus/leech/crit de equipamento | L |
| **Imbuements** | ❌ Faltando | stubs `true`; sem slots/decay/janela | XL |
| Forge (Exaltation) | ✅ Feito | fusion/transfer/dust/sliver/core/convergence/history completos | — |
| **Market** | ❌ Faltando | stub; opcodes 0xF4–0xF7 caem no default | XL |
| In-game Store / coins | 🟡 Parcial | packets roteados pro Lua gamestore; lógica toda em Lua | M |
| **Player trading** | ❌ Faltando | opcodes 0x7D–0x80 ausentes; sem safeTrade | L |
| NPC shop economy | 🟡 Parcial | buy/sell ok; ignora buyWithBackpacks/capacity | M |
| Supply stash | ❌ Faltando | stub; sem storage/protocolo stow/withdraw | L |
| Depot / locker / inbox / mailbox / reward chest | ❌ Faltando | só Store Inbox existe; sem classes de comportamento | L |
| Keys / teleport-tile / magic field / bed / trashholder | ❌ Faltando | nenhuma portada | L |

## 4. Mapa / Mundo / Protocolo — parcial (~40%)

| Subsistema | Status | Falta | Esforço |
|---|---|---|---|
| OTBM loading | 🟡 Parcial | house id e zone id **descartados**; waypoints pulados; sem mapcache; sem persistência de mapa | M |
| Tiles / flags | 🟡 Parcial | flag de casa fingida como PZ; maioria das flags ausente | M |
| **Pathfinding A\*** | ❌ Faltando | **stub — perseguição direta, sem obstáculos** (monstros/auto-walk contra parede) | L |
| Floor/z rendering | ✅ Feito | — | — |
| Sectors / spectators | 🟡 Parcial | map flat; spectators lineares **só mesmo andar** | M |
| Walking / turning / teleport | 🟡 Parcial | funciona; chase/autowalk dependem do pathfinder stub | M |
| **Stairs / floor change on step** | ❌ Faltando | não troca de andar ao pisar | M |
| MoveEvents | 🟡 Parcial | `onStepIn/Out` ok; `onEquip/DeEquip/AddItem/RemoveItem` **no-op** | M |
| **Houses** | ❌ Faltando | `House` mockClass; sem rent/auction/beds/doors/guests/serialização | XL |
| Spawns / respawn | 🟡 Parcial | funciona; sem raio/walkability/despawn | M |
| **Raids / invasions** | ❌ Faltando | `startRaid` retorna 0 | L |
| GlobalEvents | 🟡 Parcial | `onStartup` roda; **`onThink/onTime/onSave` não agendados** | M |
| Server save | ❌ Faltando | sem rotina periódica de save | M |
| Zones | ❌ Faltando | zone ids descartados | L |
| Weather / world light / Tibia time | ❌ Faltando | sem ciclo de luz/clima/hora | M |
| Login protocol | ✅ Feito | RSA/XTEA/session-key/charlist/MOTD | — |
| XTEA / RSA / checksum | ✅ Feito | — | — |
| Protocol compression | ❌ Faltando | bit reconhecido, não implementado | M |
| **Protocol recv** | 🟡 Parcial | **~27 de ~120** handlers | XL |
| **Protocol send** | 🟡 Parcial | **~36 de ~187** helpers | XL |

**Grupos de opcode faltando (recv+send):** Follow, VIP (add/edit/remove/grupos),
Channels (open/private/invite/message), Market (browse/create/cancel/accept),
Trade (request/look/accept/close), Quest log (0xF0/0xF1), Modal windows,
containers avançados (seek/browseField/parent/update), rotate/wrap/editText,
Depot/Stash, Imbuements, Highscores, reward chest, offline training, hireling
name, podium, team finder, party analyzer, bug report; e senders de Cyclopedia
(~20), world light/Tibia time, trackers (exp/loot/supply), blessings, resources
balance, e decoradores de criatura (skull/shield/emblem/icon/light).

## 5. Progressão / Cyclopedia — parcial

| Sistema | Status | Falta |
|---|---|---|
| Bestiary / Bosstiary | ✅ Feito | detalhes cosméticos (armor/mitigation/resist = 0) |
| Charms | 🟡 Parcial | Scavenge/Gut/Fatal/VoidInversion sem subsistema; Low/Savage aproximados |
| Prey / Task Hunting | 🟡 Parcial | grid fixo; dificuldade não derivada de stars |
| Wheel of Destiny | 🟡 Parcial | gems/vessels, revelation, spells do wheel inertes |
| Quest log (0xF0/0xF1) | ❌ Faltando | storages funcionam, log vazio no cliente |
| Achievements / Badges / Titles | ❌ Faltando | stubs no-op |
| Cyclopedia (11 páginas char info) | 🟡 Parcial | só Inspection (tipo 9) |
| Hirelings / Familiars / Reward chest / Daily reward | ❌ Faltando | sem engine |
| Mounts / Outfits | ✅ Feito (light) | sem gating de posse |

## 6. Persistência / Conta / Lua API — parcial

| Subsistema | Status | Falta |
|---|---|---|
| Load/Save player (~55%) | 🟡 Parcial | **NÃO salva: depot, inbox, reward chest, stash, spells aprendidos, kills/skull, condições persistentes** |
| Contas & login | 🟡 Parcial | sem 2FA, session-key, `coins_tournament`, VIP list |
| Guilds | 🟡 Parcial | só leitura; sem banco/ranks/guild wars |
| Parties | ✅ Feito | — |
| Classes Lua (~55–60%) | 🟡 Parcial | Player/Creature/Game/Monster/Spell OK; mockClass: House, Weapon, Vocation, Zone, ModalWindow, Guild, Imbuement, KV… |
| EventCallback (3/37) | ❌ Faltando | só onLogin/onLook/onMoveItem; 34 hooks no-op |
| Action/MoveEvent/TalkAction/CreatureEvent/GlobalEvent | ✅ Feito | CreatureEvent só login/death |

---

## Prioridades sugeridas

### 🔴 Crítico (dados / bugs de correção rápida)
1. Persistir **depot / inbox / reward chest / stash / spells aprendidos** (hoje somem no relog).
2. `coins_tournament` e VIP list.
3. Corrigir os bugs baratos: `uniformRandom` (haste/paralyze), `fmt.Printf` em spell, `combat_formulas.go` duplicado, mount id 388.

### 🟠 Alto impacto de gameplay
4. **A\* pathfinding real** + troca de andar ao pisar em escada/rampa (hoje monstros andam contra parede e não se sobe/desce andar caminhando).
5. **Crit + life/mana leech + reflect + absorb de player + resistências de monstro via loader Lua** (destrava Low/Savage e balanceamento).
6. **Monster spell system** completo (condições/áreas/waves/cadência) — hoje só dano.
7. **Death penalty** completa (perda de skill/mag, drop de item, downgrade) + **level-up que cresce HP/mana/cap**.
8. **Cobertura de protocolo** — priorizar canais de chat, VIP, quest log (0xF0/0xF1), trade, modal windows, cyclopedia char pages.
9. **EventCallback** (34 hooks) — muitos scripts do datapack dependem deles.

### 🟡 Sistemas grandes ainda ausentes
10. **Casas**, **Market**, **Imbuements**, **Hirelings**, **Reward/Daily reward**, **Achievements/Titles/Badges**, **Raids**, **Zones**, **World light/weather/time**.
11. Completar **Wheel** (gems/vessels/spells), **Guild** (write path + wars), classes Lua mockadas (Weapon/Vocation/ModalWindow/Imbuement…).

**Estimativa honesta:** core jogável, mas ~45–55% da superfície total do C++. As
maiores empreitadas rumo a "100%": casas, market, imbuements, monster spell
system, cobertura de protocolo (canais/VIP/trade/cyclopedia) e a superfície Lua.

---

_Auditoria line-cited dos 6 clusters em 2026-07-24 (leitura direta do código Go +
contagem de opcodes/bindings). Números são aproximados (contagem de
handlers/bindings), não exatos linha-a-linha em cada célula._
