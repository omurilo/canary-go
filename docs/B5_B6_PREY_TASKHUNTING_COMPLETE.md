# B5 & B6: Prey System & Task Hunting - Sistema COMPLETO ✅

**Data:** 2026-07-25  
**Status:** ✅ **JÁ IMPLEMENTADO** (verificado)

## Resumo

Os sistemas de **Prey** e **Task Hunting** já estão **100% implementados** no código Go, incluindo:
- 3 slots de Prey com reroll system
- 4 tipos de bônus (Damage, Defense, XP, Loot)
- Time tracking e free rerolls
- 9 slots de Task Hunting
- Boss tasks e rewards
- Protocol packets completos
- DB persistence
- Combat integration

---

## B5: PREY SYSTEM

### Arquivos Implementados

**`internal/game/prey.go`**
- PlayerPrey: gerencia 3 slots
- PreySlot: estado individual por slot
- PreyBonusType: 4 tipos (Damage, Defense, XP, Loot)
- PreyState: 6 estados (Locked, Inactive, Active, Selection, ListSelection, WildCard)

**Estruturas:**
```go
type PlayerPrey struct {
    Slots [3]*PreySlot
}

type PreySlot struct {
    ID                   byte
    State                PreyState
    BonusType            PreyBonusType
    BonusPercentage      uint16
    BonusTimeLeft        uint16          // em minutos
    FreeRerollTimeStamp  int64
    RaceIDList           []uint16        // Grid de monstros (9 opções)
    SelectedRaceID       uint16
}

// 4 Bonus Types
PreyBonus_DamageBoost      // 2*rarity + 5%
PreyBonus_DamageReduction  // 2*rarity + 10%
PreyBonus_XPBonus          // 3*rarity + 10%
PreyBonus_ImprovedLoot     // 3*rarity + 10%

// 6 States
PreyDataState_Locked         // Slot bloqueado
PreyDataState_Inactive       // Slot vazio
PreyDataState_Active         // Prey ativo
PreyDataState_Selection      // Escolhendo monster (9 grid)
PreyDataState_ListSelection  // Modo wildcard (lista completa)
PreyDataState_WildCard       // Usando wildcard
```

**Métodos:**
```go
// Reroll
GetPreyRerollPrice() uint32          // level * 200 gold
RerollPreyMonsters(slotID) bool      // Gera novo grid
SelectPreyMonster(slotID, raceID)    // Escolhe monster

// Wildcard
UsePreyCards(amount) bool            // Consome wildcard
GetPreyCards() uint32                // Retorna saldo

// Bonus
PreyBonusPercentage(type, rarity) uint16  // Calcula %
CheckPreyBonus(target) (bool, type, %)    // Verifica se aplica
```

### Database (`internal/db/prey_task.go`)

**Schema:** `player_prey` table
- Columns: player_id, slot, state, bonus_type, bonus_value, bonus_time_left, free_reroll, monster_list, selected_raceid

**Operações:**
```go
LoadPlayerPrey(ctx, p) error         // Carrega 3 slots
SavePlayerPrey(ctx, p) error         // Salva 3 slots
encodeRaceList([]uint16) []byte      // Serializa grid
decodeRaceList([]byte) []uint16      // Deserializa grid
```

### Protocol (`internal/protocol/prey_handlers.go`)

**Packets:**
```go
SendPreyData(slot)                   // Opcode: envia estado do slot
SendPreyFreeRerollTimers()           // Opcode: timers de free reroll
parsePreyAction(r)                   // Handler de ações do cliente
```

**Ações Suportadas:**
- 0: ListReroll (reroll do grid de 9)
- 1: BonusReroll (reroll do bonus type)
- 2: MonsterSelection (escolhe monster)
- 3: ListAll (wildcard mode - lista completa)

**Combat Integration:**
- Damage boost/reduction aplicado automaticamente
- XP bonus aplicado no gain experience
- Loot bonus aplicado no drop calculation

---

## B6: TASK HUNTING SYSTEM

### Arquivos Implementados

**`internal/game/task_hunter.go`**
- PlayerTaskHunter: gerencia 9 slots
- TaskSlot: task individual
- TaskHuntingState: 6 estados
- Difficulty tiers: Easy/Medium/Hard

**Estruturas:**
```go
type PlayerTaskHunter struct {
    Slots  [9]*TaskSlot
    Points uint32              // Task hunting points acumulados
}

type TaskSlot struct {
    ID                   byte
    State                TaskHuntingState
    SelectedRaceID       uint16
    Upgrade              bool            // Boss task?
    Rarity               byte            // 1-5 (bestiary stars)
    CurrentKills         uint16
    RequiredKills        uint16
    FreeRerollTimeStamp  int64
    RaceIDList           []uint16        // Grid de monstros
}

// 6 States
PreyTaskDataState_Locked        // Slot bloqueado
PreyTaskDataState_Inactive      // Slot vazio
PreyTaskDataState_Selection     // Escolhendo monster
PreyTaskDataState_ListSelection // Wildcard mode
PreyTaskDataState_Active        // Task ativa
PreyTaskDataState_Completed     // Pronto para claim

// 3 Difficulties (baseado em bestiary stars)
TaskDifficultyEasy   (≤1 star): 30-150 kills
TaskDifficultyMedium (2-3 star): 50-250 kills  
TaskDifficultyHard   (≥4 star): 100-500 kills
```

**Métodos:**
```go
// Task Management
StartTask(raceID, difficulty, rarity, upgrade)
AddTaskKill(slotID) bool               // +1 kill, retorna se completou
ClaimReward() (reward, ok)             // Calcula reward baseado em rarity

// Reroll
GetTaskHuntingRerollPrice() uint32     // level * 200 gold

// Rewards
GetTaskHuntingPoints() uint32
AddTaskHuntingPoints(amount)

// Kill Targets (baseado em difficulty + rarity)
GetTaskKillTarget(difficulty, rarity) uint16
```

**Reward Formula:**
```go
// Base reward = stars² * 1000 * difficulty_mult
// Difficulty mult: Easy=1, Medium=2, Hard=4
// Boost: 10-20% extra (chance baseado em rarity)
// Example: 5-star Hard = 5² * 1000 * 4 = 100,000 base
```

### Database (`internal/db/prey_task.go`)

**Schema:** `player_taskhunt` table
- Columns: player_id, slot, state, raceid, upgrade, rarity, kills, disabled_time, free_reroll, monster_list

**Operações:**
```go
LoadPlayerTaskHunter(ctx, p) error   // Carrega 9 slots
SavePlayerTaskHunter(ctx, p) error   // Salva 9 slots
```

### Protocol (`internal/protocol/task_hunter_handlers.go`)

**Packets:**
```go
SendTaskHuntingData(slot)            // Opcode 0xBB: estado do slot
SendTaskHuntingBasicData()           // Overview de todos slots
parseTaskHuntingAction(r)            // Handler de ações
```

**Ações Suportadas:**
- 0: ListReroll (reroll do grid)
- 1: RewardsReroll (reroll upgrade/boss flag)
- 2: ListAll (wildcard mode)
- 3: MonsterSelection (escolhe monster)
- 4: Cancel (cancela task)
- 5: Claim (reivindica reward)

**Kill Tracking:**
```go
// Automático ao matar monster
func onKill(player, target) {
    taskHunter := player.GetTaskHunter()
    for i := 0; i < 9; i++ {
        slot := taskHunter.GetSlot(i)
        if slot.State == Active && slot.SelectedRaceID == target.RaceID {
            if slot.AddKill() {
                player.SendTextMessage("Task completed!")
                slot.State = Completed
            }
        }
    }
}
```

---

## Player Integration

**Campos em `Player`:**
```go
Prey       *PlayerPrey         // 3 slots
TaskHunter *PlayerTaskHunter   // 9 slots
PreyCards  uint32              // Wildcard currency (prey_wildcard)
```

**Métodos em `Player`:**
```go
// Prey
GetPrey() *PlayerPrey
UsePreyCards(amount) bool
AddPreyCards(amount)

// Task Hunting
GetTaskHunter() *PlayerTaskHunter
GetTaskHuntingPoints() uint32
AddTaskHuntingPoints(amount)
```

---

## Sistema de Costs

### Prey Costs
- **List Reroll:** level * 200 gold (ou free a cada 20h)
- **Bonus Reroll:** 1 prey card
- **Wildcard Mode:** 5 prey cards

### Task Hunting Costs
- **List Reroll:** level * 200 gold (ou free a cada 20h)
- **Rewards Reroll:** 1 prey card
- **Wildcard Mode:** 5 prey cards

**Configuráveis via `config.lua`:**
```lua
preyRerollPricePerLevel = 200
taskHuntingRerollPricePerLevel = 200
preyBonusRerollPrice = 1
taskHuntingBonusRerollPrice = 1
preySelectListPrice = 5
taskHuntingSelectListPrice = 5
preyFreeRerollTime = 20*60*60  -- 20 horas
taskHuntingFreeRerollTime = 20*60*60
```

---

## Combat Integration

### Prey Bonuses (Automático)
```go
// Damage boost
if prey.Active && prey.BonusType == DamageBoost {
    damage *= (1 + prey.BonusPercentage/100.0)
}

// Damage reduction
if prey.Active && prey.BonusType == DamageReduction {
    damage *= (1 - prey.BonusPercentage/100.0)
}

// XP bonus
if prey.Active && prey.BonusType == XPBonus {
    exp *= (1 + prey.BonusPercentage/100.0)
}

// Loot bonus
if prey.Active && prey.BonusType == ImprovedLoot {
    lootChance *= (1 + prey.BonusPercentage/100.0)
}
```

---

## Exemplos de Uso

### Prey - Reroll via NPC
```lua
function onPreyReroll(player, slotID)
    local prey = player:getPrey()
    local slot = prey:getSlot(slotID)
    
    if slot:getState() == PREY_STATE_SELECTION then
        local price = player:getPreyRerollPrice()
        if player:removeMoney(price) then
            slot:reroll()
            player:sendPreyData(slotID)
            return true
        end
    end
    return false
end
```

### Task Hunting - Kill Tracking
```lua
function onKill(player, target)
    local taskHunter = player:getTaskHunter()
    local targetRaceID = target:getType():raceId()
    
    for i = 0, 8 do
        local slot = taskHunter:getSlot(i)
        if slot and slot:isActive() and slot:getRaceID() == targetRaceID then
            if slot:addKill() then
                player:sendTextMessage(MESSAGE_EVENT_ADVANCE, "Task completed!")
            end
        end
    end
end
```

### Claim Task Reward
```lua
function onClaimTask(player, slotID)
    local taskHunter = player:getTaskHunter()
    local slot = taskHunter:getSlot(slotID)
    
    if slot:getState() == TASK_STATE_COMPLETED then
        local reward = slot:calculateReward()
        player:addTaskHuntingPoints(reward)
        slot:reset()
        player:sendTextMessage(MESSAGE_EVENT_ADVANCE, 
            "You received " .. reward .. " task hunting points!")
        return true
    end
    return false
end
```

---

## Estado do Sistema

**Componentes Completos:**
- ✅ Core logic (prey.go, task_hunter.go)
- ✅ DB persistence (player_prey, player_taskhunt tables)
- ✅ Protocol packets (prey_handlers.go, task_hunter_handlers.go)
- ✅ Combat integration (damage/xp/loot bonuses)
- ✅ Time tracking (free rerolls a cada 20h)
- ✅ Wildcard system (prey cards)
- ✅ Boss tasks (upgrade flag)
- ✅ Reward calculation

**Features:**
- ✅ 3 Prey slots + 9 Task Hunting slots
- ✅ Grid de 9 monstros aleatórios
- ✅ Wildcard mode (lista completa)
- ✅ Free rerolls com cooldown
- ✅ Gold/Prey card costs
- ✅ Dynamic difficulty (baseado em bestiary stars)
- ✅ Reward scaling (rarity + difficulty)
- ✅ Kill tracking automático

---

## Conclusão

Os sistemas **B5 (Prey)** e **B6 (Task Hunting)** estão **100% implementados e funcionais**. O código Go possui paridade completa com o C++ original, incluindo:

- Todas as fórmulas de cálculo (bonus %, rewards)
- Sistema de reroll com costs dinâmicos
- Time tracking e free rerolls
- Protocol packets completos
- Combat integration automática
- DB persistence

**Nenhuma implementação adicional é necessária.** Os sistemas estão prontos para uso.
