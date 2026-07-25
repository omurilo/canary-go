# Plano de Implementação: Canary-Go

Este plano detalha os próximos passos para a migração do Canary (C++) para o Canary-Go, com base na auditoria de gaps (gap-analysis.md). As tarefas estão organizadas por prioridade, visando a estabilidade e a completude do servidor.

## Fase 1: Correções Críticas (Bugs de Baixo Esforço)
*Estas tarefas são de correção imediata e têm alto impacto na estabilidade.*

- [ ] **Randomização de Haste/Paralyze**: Corrigir `ConditionSpeedStruct.uniformRandom` (`internal/game/combat/condition.go:276-281`) que sempre retorna o valor máximo.
- [ ] **Limpeza de Debug**: Remover `fmt.Printf` no hot path de spells (`internal/game/spell_combat.go:139`).
- [ ] **Consolidação de Fórmulas de Combate**: Avaliar e remover/integrar `combat_formulas.go` para evitar duplicidade de fórmulas de combate com o engine principal.
- [ ] **Correção de Mount Hardcoded**: Corrigir o toggle-mount (`internal/protocol/outfit_handlers.go:201-214`) para usar as mounts reais do player, removendo o hardcode do ID 388.

## Fase 2: Persistência de Dados (Crítico)
*Evitar perda de dados do jogador entre sessões (relog).*

- [ ] **Persistência de Depot, Inbox e Reward Chest**: Implementar rotinas de salvamento e carregamento no banco de dados.
- [ ] **Persistência de Stash e Spells Aprendidos**: Garantir que o supply stash e as magias não sejam perdidos.
- [ ] **Condições Persistentes e Skulls/Kills**: Salvar estado de skulls e condições (ex: poison prolongado).
- [ ] **VIP List e Tournament Coins**: Implementar tabela e carregamento (`coins_tournament`, VIP).

## Fase 3: Alto Impacto de Gameplay (Médio/Alto Esforço)
*Sistemas que afetam diretamente a experiência central do jogo.*

- [ ] **Pathfinding A\* Real**: Substituir o stub atual por um A* com detecção de obstáculos para evitar que monstros e auto-walk andem contra as paredes.
- [ ] **Troca de Andar (Stairs/Ramps)**: Implementar a mudança automática do `Z-axis` (andar) ao pisar em escadas, rampas e buracos apropriados (`onStep`).
- [ ] **Sistema de Spells de Monstros (Monster Spell System)**: Expandir o sistema atual (que só causa dano) para suportar condições, áreas, waves, radius, e delays corretos.
- [ ] **Combate Avançado de Players**:
  - Implementar Crit chance/damage.
  - Implementar Life/Mana leech.
  - Implementar Reflect e Absorb (elemental resistance do jogador).
- [ ] **Level-up e Death Penalty**:
  - Garantir que upar de level aumente HP, Mana e Capacidade corretamente.
  - Implementar perda de skills, magic level, drop de itens e downgrade de stats ao morrer.
- [ ] **Resistências de Monstros (Lua)**: Corrigir o loader Lua para parsear corretamente `monster.elements`, garantindo imunidades e fraquezas.

## Fase 4: Protocolo e Lua API (Esforço Contínuo)
*Expandir a comunicação com o cliente e a compatibilidade com scripts do datapack.*

- [ ] **Cobertura de Protocolo**:
  - Canais de Chat e Private Messages.
  - VIP System (Add/Edit/Remove/Groups).
  - Quest Log (Opcodes 0xF0/0xF1).
  - Trade (Request/Look/Accept/Close).
  - Modal Windows.
  - Cyclopedia (Páginas do Character Info).
- [ ] **Lua API - EventCallbacks**:
  - Implementar os 34 hooks de `EventCallback` que estão faltando (apenas 3 de 37 existem atualmente), pois muitos scripts dependem deles.
- [ ] **Server Save Automático**: Implementar a rotina periódica de `GlobalEvents` (`onThink`, `onTime`, `onSave`) para realizar o server save seguro.

## Fase 5: Sistemas Complexos (Grande Esforço)
*Sistemas que demandam modelagem de dados e arquitetura dedicada.*

- [ ] **Market System**: Opcode 0xF4–0xF7 e lógica de ofertas assíncronas.
- [ ] **Imbuements**: Engine de imbuement, slots, UI no client e decaimento temporal do imbuement.
- [ ] **Houses (Casas)**: Leilões, camas, portas com chaves, convidados e persistência de itens dentro das casas.
- [ ] **Reward Chest & Daily Rewards**: Engine e interface do cliente.
- [ ] **Wheel of Destiny (Fase Final)**: Ativar gems/vessels e spells da Wheel (atualmente inertes).
- [ ] **Achievements, Badges e Titles**: Integração de progresso e exibição.
- [ ] **Guilds**: Escrita de dados, bank de guild e suporte a Guild Wars.
- [ ] **Ambiente (Weather/Time/Light)**: Ciclos de dia e noite, clima e sincronização de luz global.

---
**Próximos passos imediatos recomendados**: Iniciar a Fase 1 e, paralelamente, planejar o esquema de banco de dados e serialização para a Fase 2 (Depot/Inbox).
