# Plano de Implementação — Sistemas Pendentes

## 1. Forge System (`GetForgeSkillStat`)

**Descrição:** O sistema Forge permite aprimorar equipamentos com dust/slivers/cores,
concedendo bônus como Onslaught (slot left), Dodge (slot armor), Momentum (slot head),
e bônus nos slots legs/feet.

**Dependências:**
- Tabela `player_forge` (já tem `forge_dusts`, `forge_dust_level` no players)
- Sistema de items forgeáveis (forge_sliver, forge_core)

**Arquivos:**
- `internal/game/forge.go` — struct `ForgeData` (dusts por slot, níveis)
- `internal/db/player.go` — Load/Save forge data
- `internal/game/player_forge.go` — métodos: `GetForgeSkillStat(slot)`
- Schema: `player_forge` table ou colunas JSON em `players`

**Dados:**
```go
type ForgeData struct {
    Slots [9]ForgeSlot  // 9 equip slots
}
type ForgeSlot struct {
    DustLevel int32   // 0-100
    DustCount int32
}
```

**Cálculo do bônus:** `dustLevel * 0.01` (1% por nível de dust)

**Estimativa:** Média (2-3 dias)

---

## 2. Weapon Proficiency — Perk System Completo

**Descrição:** O sistema de perks por arma (níveis 1-5, cada nível 3 perks).
O esqueleto já existe (`weapon_proficiency.go`), falta:
1. Carregar as definições de perks do JSON (C++ carrega de `data/weapon_proficiency.json`)
2. Implementar a seleção de perks por arma
3. Calcular stats a partir das perks selecionadas
4. Salvar no KV

**Arquivos:**
- `internal/game/weapon_proficiency.go` — adicionar `Proficiency`, `ProficiencyLevel`, `ProficiencyPerk`
- `internal/game/weapon_proficiency_data.go` — loader do JSON de definições
- `internal/game/weapon_proficiency_calc.go` — cálculo de stats das perks
- `data/weapon_proficiency.json` — definições das perks (portar do C++)

**Estruturas:**
```go
type Proficiency struct {
    ID    uint16
    Name  string
    Levels []ProficiencyLevel
}
type ProficiencyLevel struct {
    Perks []ProficiencyPerk
    ExpRequired uint32
}
type ProficiencyPerk struct {
    Type  WeaponProfBonus  // ATTACK_DAMAGE, DEFENSE, etc.
    Value float64
}
```

**Estimativa:** Alta (3-5 dias) — inclui parser JSON, cálculo, UI de seleção

---

## 3. Food Tracking (`GetActiveFoods`)

**Descrição:** Rastrear items de food ativos (com tempo restante).
O player já tem `RegenTicks int32` (tempo total de regeneração).

**Dependências:**
- Sistema de condições (CONDITION_REGENERATION)

**Arquivos:**
- `internal/game/player.go` — adicionar `Foods map[uint16]int64` (itemID → expiry)
- `internal/game/player_food.go` — métodos: `AddFood`, `GetActiveFoods`, `ConsumeFood`
- `internal/db/player.go` — Load/Save foods

**Dados:**
```go
type ActiveFood struct {
    ItemID   uint16
    TimeLeft uint32 // seconds
}
```

**Estimativa:** Baixa (1 dia) — simililar a `GetActiveConcoctions`

---

## 4. Wheel Augments (`GetWheelAugments`)

**Descrição:** A Wheel of Destiny tem augments que afetam stats.
O sistema Wheel já existe parcialmente (`wheel.go`, `wheel_handlers.go`).

**Dependências:**
- Wheel of Destiny completo (já existe esqueleto)
- Sistema de augments do wheel

**Arquivos:**
- `internal/game/wheel.go` — adicionar método `GetActiveAugments() []WheelAugment`
- `internal/game/wheel_augments.go` — definições de augments por slot

**Dados:**
```go
type WheelAugment struct {
    Slot uint8
    Type uint8
    Value float64
}
```

**Estimativa:** Média (2 dias)

---

## 5. Equipped Augments (`GetEquippedAugments`)

**Descrição:** Augments de items equipados (encantamentos especiais).
Similar a imbuements mas com dados diferentes.

**Dependências:**
- Sistema de augments por item (similar a imbuements)
- Coluna em `player_items` ou KV store

**Arquivos:**
- `internal/game/item_augments.go` — structs e métodos
- `internal/protocol/game_augments.go` — handlers de opcode

**Estimativa:** Alta (3-4 dias) — sistema novo com persistência

---

## 6. Distance Accuracy (`GetDamageAccuracy`)

**Descrição:** Precisão de armas distance — retorna `[]float64` com
a chance de acerto por alcance (1-7).

**Dependências:**
- Item type tem stats de accuracy (ou fórmula)
- Fórmula C++: `weaponHitChance - (range - 2) * 4` (mínimo 30)

**Arquivos:**
- `internal/game/player_stats.go` — implementar `GetDamageAccuracy`

**Fórmula:**
```go
func (p *Player) GetDamageAccuracy(item *Item) []float64 {
    // HitChance do item (default 85 para distance)
    // Penalidade de -4 por range além de 2
    // Mínimo 30
}
```

**Estimativa:** Baixa (4 horas)

---

## 7. Mitigation (`GetMitigation`)

**Descrição:** Valor de mitigação de dano do jogador.
Calculado do vocation.xml + shield skill.

**Dependências:**
- Parse do `<mitigation>` no vocations.xml (já existe no XML, falta no struct Go)

**Arquivos:**
- `internal/game/vocations/vocation.go` — adicionar `Mitigation` struct no Vocation
- `internal/luaengine/vocation.go` — expor mitigation pro Lua se necessário
- `internal/game/player_stats.go` — implementar `GetMitigation`

**Cálculo:**
```
mitigation = vocation.mitigation.multiplier * (shieldMitigation / 10000)
onde shieldMitigation = primaryShield * min(skill, 200) / 200 + secondaryShield
```

**Estimativa:** Baixa (4-6 horas)

---

## Ordem Recomendada

| Prioridade | Sistema | Justificativa |
|------------|---------|---------------|
| 1 | **Mitigation** | Rápido, desbloqueia campo na cyclopedia |
| 2 | **Distance Accuracy** | Rápido, afeta OffenceStats |
| 3 | **Food Tracking** | Médio, desbloqueia MiscStats |
| 4 | **Wheel Augments** | Médio, desbloqueia MiscStats |
| 5 | **Weapon Proficiency Perks** | Complexo, desbloqueia vários campos |
| 6 | **Forge** | Complexo, desbloqueia vários campos |
| 7 | **Equipped Augments** | Complexo, sistema novo |

**Total estimado:** ~14-20 dias de trabalho.