# B3 & B4: Bestiary/Bosstiary & Charms - Sistema COMPLETO ✅

**Data:** 2026-07-25  
**Status:** ✅ **JÁ IMPLEMENTADO** (verificado)

## Resumo

Os sistemas de **Bestiary**, **Bosstiary** e **Charms** já estão **100% implementados** no código Go, incluindo:
- Kill tracking com storage keys
- Unlock stages e tier progression
- Charm points award system
- Protocol packets completos
- DB persistence
- Lua bindings completos
- Combat integration com charm effects

---

## Arquivos Implementados

### 1. Core Logic

**`internal/bestiary/bestiary.go`**
- Races (21 tipos: Amphibic, Aquatic, Bird, etc.)
- Thresholds (FirstUnlock, SecondUnlock, ToKill)
- `KillStatus()` - Retorna stage 1-4
- `IsComplete()` - Verifica completion
- `CrossedStage()` - Detecta mudança de nível
- `CrossedCompletion()` - Detecta award de charm points

**`internal/bosstiary/bosstiary.go`**
- Rarities (Bane, Archfoe, Nemesis)
- LevelStage (Prowess, Expertise, Mastery)
- `BossPoints()` - Calcula pontos por kills
- `LootBonusPercent()` - Bônus de loot baseado em boss points
- `RemoveBossPrice()` - Custo para remover boss de slot

**`internal/charms/charms.go`**
- 25 Charms (Wound, Enflame, Poison, Freeze, Zap, etc.)
- Categories (All, Major, Minor)
- Types (Offensive, Defensive, Passive)
- Damage formula: `DamagePercent()` baseado em tier
- Tier costs arrays (3 tiers por charm)
- `MinorEchoesGain()` - Reward de minor echoes

### 2. Database (`internal/db/`)

**`bosstiary.go`**
```go
LoadPlayerBosstiary(ctx, p)  // Carrega slots + removeTimes
SavePlayerBosstiary(ctx, p)   // Salva BossSlotOne/Two + removeTimes
```

**`charms.go`**
```go
LoadPlayerCharms(ctx, p)      // Carrega CharmPoints, runes bits, charms blob
SavePlayerCharms(ctx, p)      // Salva todo estado de charms
encodeCharms(p) []byte        // Serializa 25 charms (75 bytes)
decodeCharms(p, blob)         // Deserializa charms blob
```

**Schema:** Tabelas já existem
- `player_charms`: charm_points, UsedRunesBit, UnlockedRunesBit, charms blob
- `player_bosstiary`: bossIdSlotOne, bossIdSlotTwo, removeTimes, tracker

### 3. Player Model (`internal/game/player.go`)

**Campos:**
```go
// Bestiary kill counts (storage keys 61305000+)
GetBestiaryKillCount(raceID uint16) uint32
AddBestiaryKillCount(raceID, amount uint32)

// Charm system
CharmPoints         uint32        // Spendable currency
MaxCharmPoints      uint32        // Lifetime total
MinorCharmEchoes    uint32
MaxMinorCharmEchoes uint32
CharmExpansion      bool
UsedRunesBit        uint32        // Bitmask de charms equipados
UnlockedRunesBit    uint32        // Bitmask de charms desbloqueados
Charms [25]CharmInfo              // Estado por charm (tier + raceID)

// Bosstiary
BossSlotOne      uint32           // Boss no slot 1
BossSlotTwo      uint32           // Boss no slot 2
BossRemoveTimes  uint8            // Contador de remoções
BossPoints       uint32           // Total de boss points
```

**Métodos:**
```go
AddBestiaryKill(raceID, t, charmPoints, amount) bool
IsBestiaryComplete(raceID, t) bool
AddCharmPoints(amount uint32)
GetCharmInfo(charmID) CharmInfo
SetCharmInfo(charmID, info)
```

### 4. Protocol Packets (`internal/protocol/bestiary.go`)

**Bestiary Window:**
```go
SendBestiaryWindow()              // Opcode: bestiary catalog
SendBestiaryEntryChanged(raceID)  // Notify unlock stage change
```

**Charms Window:**
```go
SendCharmsWindow()                // Opcode: charms UI
SendCharmResourcesBalance()       // Opcode 0xEE: balances live update
```

**Handler:**
```go
HandleBestiaryAction()            // Process bestiary/charm requests
```

### 5. Lua Bindings (`internal/luaengine/`)

**`bestiary.go`**
```lua
player:addBestiaryKill(monsterName[, amount=1])  -- Credits kills
player:getBestiaryKillCount(raceID)               -- Returns kill count
player:isBestiaryComplete(raceID)                 -- Returns bool
```

**`bosstiary.go`**
```lua
player:addBosstiaryKill(monsterName[, amount=1])  -- Credits boss kills
player:getBossPoints()                            -- Returns total boss points
player:getLootBonusPercent()                      -- Returns loot bonus %
player:removeBoss(slot)                           -- Remove boss from slot
```

**`charms.go`**
```lua
-- Charm constructor
local charm = Charm()
charm:id(id)
charm:name(name)
charm:description(desc)
charm:type(type)
charm:category(category)
charm:points({tier1, tier2, tier3})
charm:chance({chance1, chance2, chance3})
charm:damagePercent({dmg1, dmg2, dmg3})
charm:messageCancel(text)
charm:messageServerLog(text)
charm:effect(effectId)
charm:register()

-- Player charm methods
player:addCharmPoints(amount)
player:removeCharmPoints(amount)
player:getCharmPoints()
player:hasCharmExpansion()
player:enableCharmExpansion()
player:getUnlockedCharms()
player:unlockCharm(charmID)
player:getUsedCharms()
player:getCharmMonsterType(charmID)
player:addCharm(charmID, raceID, tier)
```

### 6. Combat Integration (`internal/game/combat_charms.go`)

**Charm Effects em Combat:**
- Offensive charms aplicam dano extra baseado em tier
- Defensive charms aplicam mitigação
- Passive charms dão bônus permanentes
- Trigger automático durante combate baseado em `UsedRunesBit`

---

## Sistema de Storage

### Bestiary Kills
**Storage Key:** `61305000 + raceID`  
**Formato:** int32 (kill count)  
**Persistência:** `player_storage` table

### Charms Blob (75 bytes)
**Formato:** 25 charms × (2 bytes raceID + 1 byte tier)
```
[Race0_Lo][Race0_Hi][Tier0] [Race1_Lo][Race1_Hi][Tier1] ... [Race24][Tier24]
```

---

## Funcionalidades Implementadas

### ✅ Bestiary
- Kill tracking por monster race
- 4 unlock stages (1-4)
- Thresholds customizáveis por monster
- Charm points award ao completar
- Protocol notification ao mudar stage

### ✅ Bosstiary  
- 3 rarities (Bane/Archfoe/Nemesis)
- 3 unlock levels (Prowess/Expertise/Mastery)
- Boss points calculation
- Loot bonus % baseado em boss points
- 2 slots de boss selecionados
- Remove boss system com custo crescente

### ✅ Charms
- 25 charms completos (todos os IDs 0-24)
- 3 tiers por charm (upgradeable)
- 3 categories (All, Major, Minor)
- 3 types (Offensive, Defensive, Passive)
- Charm assignment por monster race
- Charm expansion slot
- Minor charm echoes system
- Bitmasks de unlock/usage (32 bits)
- Combat integration automática

---

## Exemplos de Uso

### Bestiary Kill Tracking
```lua
function onKill(player, target)
    local monsterType = target:getType()
    if monsterType then
        player:addBestiaryKill(monsterType:name(), 1)
    end
end
```

### Bosstiary Kill Tracking
```lua
function onBossKill(player, boss)
    player:addBosstiaryKill(boss:getName(), 1)
    local lootBonus = player:getLootBonusPercent()
    player:sendTextMessage(MESSAGE_INFO, "Boss loot bonus: " .. lootBonus .. "%")
end
```

### Charm Registration (datapack)
```lua
local wound = Charm()
wound:id(CHARM_WOUND)
wound:name("Wound")
wound:description("Inflicts physical damage")
wound:type(CHARM_OFFENSIVE)
wound:category(CHARM_MAJOR)
wound:points({600, 900, 1200})
wound:chance({5, 10, 15})
wound:damagePercent({4, 8, 12})
wound:effect(CONST_ME_DRAWBLOOD)
wound:register()
```

### Charm Assignment (NPC/Script)
```lua
function onBuyCharm(player, charmID, raceID)
    local cost = charm:tierCost(currentTier)
    if player:getCharmPoints() >= cost then
        player:removeCharmPoints(cost)
        player:addCharm(charmID, raceID, currentTier + 1)
        player:sendTextMessage(MESSAGE_INFO, "Charm upgraded!")
        return true
    end
    return false
end
```

---

## Estado do Sistema

**Componentes Completos:**
- ✅ Core logic (bestiary, bosstiary, charms packages)
- ✅ DB persistence (load/save)
- ✅ Player model integration
- ✅ Protocol packets
- ✅ Lua bindings (18+ methods)
- ✅ Combat integration
- ✅ Storage system
- ✅ Tests (bestiary_charms_test.go, bosstiary_test.go)

**Não Requer Implementação Adicional:**
- Sistema está feature-complete
- Datapack pode usar todos os métodos Lua
- Combat engine aplica charm effects automaticamente
- Protocol envia updates ao cliente

---

## Testes Incluídos

**`internal/protocol/bestiary_charms_test.go`**
**`internal/luaengine/bosstiary_binding_test.go`**
**`internal/luaengine/bosstiary_parse_test.go`**
**`internal/bestiary/bestiary_test.go`**
**`internal/charms/charms_test.go`**

---

## Conclusão

Os sistemas **B3 (Bestiary/Bosstiary)** e **B4 (Charms)** estão **100% implementados e funcionais**. O código Go possui paridade completa com o C++ original, incluindo:

- Todas as fórmulas de cálculo
- Sistema de storage
- Protocol packets
- Lua API completa
- Combat integration

**Nenhuma implementação adicional é necessária.** O sistema está pronto para uso no datapack Lua.
