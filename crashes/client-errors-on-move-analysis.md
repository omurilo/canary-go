# Análise — Erros de Movimento de Criaturas (OTClient)

## Metadados

| Campo | Valor |
|---|---|
| **Data** | 30/07/2026 |
| **Período** | 20:50:46 → 21:02:28 (~12 minutos) |
| **Total de erros** | 439 `parseCreatureMove` + 354 `no thing at pos` |
| **Frequência** | ~1 erro a cada 1-2 segundos |
| **Posições** | X: 32361-32386, Y: 32157-32246, Z: 7 |
| **stackpos** | 1-4 |

---

## Erro

```
[error] no thing at pos:X,Y,Z, stackpos:N
[error] ProtocolGame::parseCreatureMove: no creature found to move
```

O primeiro erro é do sistema de tile do OTClient: ao acessar a stack position N no tile (X,Y,Z), não encontra nada.
O segundo erro é do parser: ao processar `opCreatureMove` (0x6D), a criatura não está onde o servidor disse que estaria.

---

## Causa Raiz

### 1. `StackPosWithIndex` calcula stack position incorreta

**Arquivo:** `internal/protocol/game_encode.go:351`

```go
func (g *GameProtocol) StackPosWithIndex(pos game.Position, tileIndex int) uint8 {
    stack := 0
    tile := g.deps.World.Map.GetTile(pos)
    if tile != nil {
        if tile.Ground != nil { stack++ }
        for _, it := range tile.Items {
            if g.isTopItem(it) { stack++ }
        }
        if tileIndex >= 0 {
            stack += len(tile.Creatures) - tileIndex
        }
    }
    return uint8(stack)
}
```

O cálculo `len(tile.Creatures) - tileIndex` conta as criaturas que estavam ACIMA da removida. Se `tileIndex = 0` (primeira criatura da lista), o resultado é `len(tile.Creatures)` — todas as criaturas restantes. Isso está correto APENAS se a lista de criaturas não mudou desde a remoção.

### 2. TOCTOU Race Condition

**Arquivo:** `internal/game/world.go:596-627`

Em `TryMoveCreature`:

```go
w.mu.Lock()
oldTileIndex := w.removeCreatureFromTile(c)  // A: calcula índice
c.SetPosition(dest)
w.addCreatureToTile(c)
w.mu.Unlock()

if w.OnCreatureMove != nil {
    w.OnCreatureMove(c, oldPos, dest, oldTileIndex)  // B: usa índice
}
```

Entre **A** e **B** o mutex é liberado. Outra goroutine pode modificar `tile.Creatures` (adicionar/remover criaturas) ANTES de `StackPosWithIndex` travar o RLock, causando:

```
Tile tinha: [MonstroA, MonstroB, Player]
tileIndex = 0 (MonstroA removido)

Antes de StackPosWithIndex travar:
Outra goroutine adiciona MonstroC: [MonstroB, Player, MonstroC]
len = 3, tileIndex = 0
stack += 3 - 0 = 3 ← ERRADO (deveria ser 2)
```

### 3. `removeCreatureFromTile` pode retornar -1

**Arquivo:** `internal/game/world.go:431`

```go
func (w *World) removeCreatureFromTile(c Creature) int {
    t := w.Map.GetTile(c.GetPosition())
    if t != nil {
        for i, v := range t.Creatures {
            if v.GetID() == c.GetID() {
                t.Creatures = append(t.Creatures[:i], t.Creatures[i+1:]...)
                return i
            }
        }
    }
    return -1
}
```

Se a criatura NÃO for encontrada no tile (já foi removida, posição incorreta, etc.), retorna `-1`. Quando `tileIndex = -1`, a stack position pula a contagem de criaturas:

```go
if tileIndex >= 0 {
    stack += len(tile.Creatures) - tileIndex
}
```

Resultado: stack position = apenas ground + top items, sem criaturas → cliente encontra "no thing" na stack, mesmo a criatura estando lá em cima.

### 4. Ordem das criaturas no slice

`addCreatureToTile` faz `append` no final. `removeCreatureFromTile` remove por ID. A ordem das criaturas no slice define qual stack position cada uma ocupa.

Se a ordem esperada pelo cliente (ground → top items → criaturas de baixo para cima) não corresponder à ordem no slice, o stack position calculado não vai bater com o que o cliente espera.

---

## Impacto

- Cliente recebe pacotes de movimento inválidos
- Criaturas "teleportam" visualmente em vez de andar suavemente
- Em casos extremos, pode causar crash se o cliente tentar acessar um índice inválido
- Spam de ~30 erros/minuto no log do cliente

---

## Correções Recomendadas

### 1. Proteger `OnCreatureMove` com o mutex

Mover a chamada de `OnCreatureMove` para dentro do `w.mu.Lock()` em `TryMoveCreature`:

```go
w.mu.Lock()
oldTileIndex := w.removeCreatureFromTile(c)
c.SetPosition(dest)
w.addCreatureToTile(c)
if w.OnCreatureMove != nil {
    w.OnCreatureMove(c, oldPos, dest, oldTileIndex)
}
w.mu.Unlock()
```

Ou, se for muito custoso, usar `w.mu.RLock()` no `BroadcastCreatureMove` E em toda a cadeia de `StackPosWithIndex`.

### 2. Log quando `removeCreatureFromTile` retorna -1

Adicionar log em `TryMoveCreature` para capturar quando `oldTileIndex == -1`:

```go
oldTileIndex := w.removeCreatureFromTile(c)
if oldTileIndex == -1 {
    slog.Default().Warn("creature not found in tile",
        "id", c.GetID(), "pos", c.GetPosition())
}
```

### 3. Verificar isKnown + spectator

Confirmar que `BroadcastCreatureMove` só envia movimento para criaturas que o cliente já conhece (`gp.isKnown`). A verificação existe na linha 61, mas precisa ser confirmada.

---

## Arquivos Afetados

```
internal/game/world.go:596-627        # TryMoveCreature — race condition
internal/game/world.go:431-442        # removeCreatureFromTile — retorna -1
internal/protocol/game_encode.go:351-370  # StackPosWithIndex — cálculo
internal/protocol/game_broadcast.go:35-78 # BroadcastCreatureMove — chamada
internal/protocol/game_actions.go:711-718 # SendCreatureMove — envio do pacote
```
