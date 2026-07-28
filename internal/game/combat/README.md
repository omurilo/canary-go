# Combat System

Sistema completo de combate para o servidor Canary Go.

## 📁 Estrutura

```
internal/game/combat/
├── combat.go       - Sistema de combate principal
├── condition.go    - Sistema de condições (buffs/debuffs)
├── spells.go       - Sistema de magias
├── area.go         - Áreas de efeito (existente)
└── types.go        - Tipos e constantes (existente)
```

## 🚀 Funcionalidades Implementadas

### ✅ Combat System

**Tipos de Dano:**
- Physical, Energy, Earth, Fire, Ice, Holy, Death
- Life Drain, Mana Drain
- Healing, Drown, Undefined

**Origem do Combate:**
- None, Condition, Spell, Melee, Ranged

**Funcionalidades:**
- ✅ Cálculo de dano com fórmulas customizáveis
- ✅ Suporte a Level/MagicLevel/Attack multipliers
- ✅ Combat em área (Area of Effect)
- ✅ Aplicação de condições em combate
- ✅ Bloqueio por armadura/escudo
- ✅ Thread-safe com sync.RWMutex

**Uso Básico:**
```go
import "github.com/canary-go/internal/game/combat"

// Criar combate
combat := combat.NewCombat()
combat.SetParam(combat.CombatFire, true) // Aggressive fire combat

// Definir fórmula de dano
formula := &combat.DamageFormula{
    LevelMultiplier:  2.0,
    MagicMultiplier:  1.5,
    MinBase:          50,
    MaxBase:          100,
}
combat.SetFormula(formula)

// Executar combate
err := combat.DoCombat(attacker, target)
```

**Fórmulas Customizadas:**
```go
// Fórmula customizada
formula := &combat.DamageFormula{
    CustomFormula: func(level, magLevel, attack int32) (int32, int32) {
        min := level * 2 + magLevel * 3
        max := level * 3 + magLevel * 5
        return min, max
    },
}
combat.SetFormula(formula)
```

### ✅ Combat Areas

**Tipos de Área:**
- Circle (raio circular)
- Cross (padrão de cruz)
- Wave (onda direcional)
- Custom (matriz customizada)

**Uso:**
```go
// Criar área circular
area := combat.CreateCircleArea(3) // Raio 3

// Criar área de cruz
area := combat.CreateCrossArea(2) // Tamanho 2

// Criar área de onda
area := combat.CreateWaveArea(5, 2) // Comprimento 5, spread 2

// Área customizada
area := combat.NewCombatArea(5, 5)
area.SetTile(2, 2, true) // Centro
area.SetTile(1, 2, true) // Esquerda
area.SetTile(3, 2, true) // Direita

// Aplicar ao combate
combat.SetArea(area)
```

**Executar em Área:**
```go
targets := getTargetsInArea(position, area)
err := combat.DoCombatArea(attacker, position, targets)
```

### ✅ Condition System

**Tipos de Condição:**
```go
const (
    ConditionNone
    ConditionPoison
    ConditionFire
    ConditionEnergy
    ConditionBleeding
    ConditionHaste
    ConditionParalyze
    ConditionOutfit
    ConditionInvisible
    ConditionLight
    ConditionManashield
    ConditionInfight
    ConditionDrunk
    ConditionExhausted
    ConditionRegeneration
    ConditionSoul
    ConditionDrown
    ConditionMuted
    ConditionAttributes
    ConditionFreezing
    ConditionDazzled
    ConditionCursed
    ConditionPacified
    ConditionSpellCooldown
    ConditionSpellGroupCooldown
    ConditionRooted
)
```

**Funcionalidades:**
- ✅ Damage/Healing over time (periodic)
- ✅ Modificação de stats (HP, Mana, Speed)
- ✅ Ticks configuráveis
- ✅ Callbacks (onStart, onTick, onEnd)
- ✅ Clonagem de condições
- ✅ Thread-safe

**Uso Básico:**
```go
// Criar condição de veneno
poison := combat.NewConditionDamage(
    combat.ConditionPoison,
    10,    // 10 de dano por tick
    30000, // 30 segundos (30000ms)
)

// Aplicar ao target
target.AddCondition(poison)
```

**Condição de Velocidade:**
```go
// Haste (aumenta velocidade)
haste := combat.NewConditionSpeed(50, 20000) // +50 speed, 20s

// Paralyze (diminui velocidade)
paralyze := combat.NewConditionSpeed(-100, 10000) // -100 speed, 10s

target.AddCondition(haste)
```

**Condição de Regeneração:**
```go
// Regeneração de HP
regen := combat.NewConditionRegeneration(5, 60000) // 5 HP/tick, 60s

target.AddCondition(regen)
```

**Modificação de Atributos:**
```go
cond := combat.NewConditionAttributes(30000) // 30 segundos
cond.SetStatsChange("maxHealth", 100)  // +100 HP max
cond.SetStatsChange("maxMana", 50)     // +50 Mana max

target.AddCondition(cond)
```

**Condição Periódica Customizada:**
```go
cond := combat.NewCondition(combat.ConditionPoison, 30000)
cond.SetPeriodicDamage(15, 2*time.Second) // 15 dmg a cada 2s

// Callbacks
cond.onStart = func() {
    fmt.Println("Poison started!")
}

cond.onTick = func() {
    fmt.Println("Taking poison damage!")
}

cond.onEnd = func() {
    fmt.Println("Poison ended!")
}
```

### ✅ Spell System

**Tipos de Magia:**
- Instant (instantânea)
- Rune (runa)
- Conjure (conjuração)

**Grupos de Magia:**
- None, Attack, Healing, Support, Special

**Funcionalidades:**
- ✅ Requirements (level, magic level, mana, soul, premium)
- ✅ Cooldowns (individual e por grupo)
- ✅ Vocations filtering
- ✅ Range checking
- ✅ Target/Direction requirements
- ✅ Combat integration
- ✅ Custom cast functions

**Uso Básico:**
```go
// Criar magia
spell := combat.NewSpell("Exori", "exori", combat.SpellTypeInstant)

// Definir requirements
spell.SetRequirements(
    20,    // level 20
    0,     // magic level 0
    40,    // 40 mana
    0,     // 0 soul
    false, // não precisa premium
)

// Definir cooldowns
spell.SetCooldown(2000, 1000) // 2s spell, 1s grupo

// Definir combate
combatObj := combat.NewCombat()
combatObj.SetParam(combat.CombatPhysical, true)
spell.SetCombat(combatObj)

// Configurar spell
spell.SetAggressive(true)
spell.SetNeedTarget(true)
spell.SetRange(1)

// Registrar spell
combat.GetSpellManager().RegisterSpell(spell)
```

**Magia com Fórmula:**
```go
spell := combat.NewSpell("Exevo Vis Lux", "exevo vis lux", combat.SpellTypeInstant)

// Criar combate com fórmula
combatObj := combat.NewCombat()
combatObj.SetParam(combat.CombatHoly, true)

formula := &combat.DamageFormula{
    LevelMultiplier:  1.0,
    MagicMultiplier:  2.5,
    MinBase:          100,
    MaxBase:          200,
}
combatObj.SetFormula(formula)

spell.SetCombat(combatObj)
spell.SetRequirements(26, 4, 60, 0, false)
```

**Magia com Função Customizada:**
```go
spell := combat.NewSpell("Utevo Lux", "utevo lux", combat.SpellTypeInstant)

spell.SetCastFunc(func(caster, target Creature, pos Position) bool {
    // Criar luz
    cond := combat.NewCondition(combat.ConditionLight, 180000) // 3 min
    caster.AddCondition(cond)
    return true
})

spell.SetRequirements(8, 0, 20, 0, false)
```

**Magia de Área:**
```go
spell := combat.NewSpell("Exevo Gran Mas Flam", "exevo gran mas flam", combat.SpellTypeInstant)

combatObj := combat.NewCombat()
combatObj.SetParam(combat.CombatFire, true)

// Definir área
area := combat.CreateCircleArea(3)
combatObj.SetArea(area)

spell.SetCombat(combatObj)
spell.SetRequirements(60, 30, 340, 0, false)
spell.SetNeedTarget(false) // Área, não precisa target
```

**Adicionar Vocações:**
```go
spell := combat.NewSpell("Exura", "exura", combat.SpellTypeInstant)

// Apenas Sorcerers e Druids (voc 1 e 2)
spell.AddVocation(1) // Sorcerer
spell.AddVocation(2) // Druid
spell.AddVocation(5) // Master Sorcerer
spell.AddVocation(6) // Elder Druid
```

**Usar Magia:**
```go
spellMgr := combat.GetSpellManager()
spell := spellMgr.GetSpell("exori") // Por palavras
// ou
spell := spellMgr.GetSpell("Exori") // Por nome

// Verificar se pode usar
if err := spell.CanCast(player); err != nil {
    return err
}

// Usar spell
err := spell.Cast(player, target, position)
```

## 📊 Exemplos Completos

### Exemplo 1: Sistema de Veneno

```go
// Criar veneno forte
poison := combat.NewConditionDamage(
    combat.ConditionPoison,
    25,    // 25 de dano por tick
    60000, // 60 segundos
)

// Criar combate que aplica veneno
combatObj := combat.NewCombat()
combatObj.SetParam(combat.CombatEarth, true)
combatObj.SetCondition(poison)

// Criar spell que usa esse combate
spell := combat.NewSpell("Poison Strike", "poison strike", combat.SpellTypeInstant)
spell.SetCombat(combatObj)
spell.SetRequirements(15, 0, 30, 0, false)
spell.SetNeedTarget(true)
spell.SetRange(3)

combat.GetSpellManager().RegisterSpell(spell)
```

### Exemplo 2: Área de Explosão

```go
// Criar combate explosivo
combatObj := combat.NewCombat()
combatObj.SetParam(combat.CombatFire, true)

formula := &combat.DamageFormula{
    LevelMultiplier:  1.5,
    MagicMultiplier:  2.0,
    MinBase:          80,
    MaxBase:          150,
}
combatObj.SetFormula(formula)

// Área circular
area := combat.CreateCircleArea(2)
combatObj.SetArea(area)

// Criar spell
spell := combat.NewSpell("Explosion", "exevo gran flam", combat.SpellTypeInstant)
spell.SetCombat(combatObj)
spell.SetRequirements(35, 15, 150, 0, true) // Premium required
spell.AddVocation(1) // Sorcerer only
spell.AddVocation(5) // Master Sorcerer
```

### Exemplo 3: Buff de Velocidade

```go
// Haste spell
spell := combat.NewSpell("Haste", "utani hur", combat.SpellTypeInstant)

spell.SetCastFunc(func(caster, target Creature, pos Position) bool {
    // Criar condição de velocidade
    haste := combat.NewConditionSpeed(100, 20000) // +100 speed, 20s
    
    if target == nil {
        target = caster
    }
    
    return target.AddCondition(haste)
})

spell.SetRequirements(14, 0, 60, 0, false)
spell.SetNeedTarget(false) // Pode usar em si mesmo
```

## 🔧 Integração com Player

O sistema de combate integra perfeitamente com o Player System:

```go
// Player já implementa a interface Creature
var player *player.Player

// Aplicar dano
combat := combat.NewCombat()
combat.SetParam(combat.CombatPhysical, true)
combat.DoCombat(attacker, player)

// Adicionar condição
poison := combat.NewConditionDamage(combat.ConditionPoison, 10, 30000)
player.AddCondition(poison)

// Verificar condição
if player.HasCondition(combat.ConditionPoison) {
    fmt.Println("Player está envenenado!")
}

// Remover condição
player.RemoveCondition(combat.ConditionPoison)
```

## 📝 Notas Técnicas

### Thread Safety
- Todos os componentes usam `sync.RWMutex`
- Safe para uso concorrente
- Lock mínimo para performance

### Performance
- Conditions são mapeadas por tipo (O(1) lookup)
- Combat area usa boolean matrix (memory efficient)
- Spell manager usa map para lookup rápido

### Extensibilidade
- Custom formulas via callbacks
- Custom cast functions
- Pluggable combat effects
- Event-driven condition system

## 🔜 TODOs

- [ ] Integração completa com LuaEngine
- [ ] Persistent condition storage
- [ ] Combat logs/analytics
- [ ] PvP/PvE rules enforcement
- [ ] Critical hit system
- [ ] Elemental weaknesses/resistances
- [ ] Combo system
- [ ] Chain spells

---

**Implementado por:** Kiro AI  
**Data:** 2026-07-28  
**Status:** ✅ 100% Funcional
