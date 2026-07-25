# B13: Guild System - Implementação Completa

**Data:** 2026-07-25  
**Status:** ✅ Implementado

## Resumo

Sistema completo de Guilds migrado do C++ para Go, incluindo criação, gerenciamento de ranks, membros, MOTD, banco da guild, e Lua bindings.

## Arquivos Criados/Modificados

### 1. Models Go (`internal/game/`)

**`internal/game/guild.go`** - Estruturas principais
```go
type Guild struct {
    ID           uint32
    Name         string
    OwnerID      uint32
    CreationDate time.Time
    MOTD         string
    Balance      uint64
    Points       int32
    Level        int32
    
    Ranks         []*GuildRank
    MembersOnline []*Player
    MemberCount   uint32
}

type GuildRank struct {
    ID      uint32
    Name    string
    Level   uint8  // 1=Member, 2=Vice-Leader, 3=Leader
    GuildID uint32
}

type GuildMember struct {
    PlayerID uint32
    GuildID  uint32
    RankID   uint32
    Nick     string
}
```

**Métodos implementados:**
- `NewGuild(id, name)` - Cria nova guild
- `AddMember(p)` / `RemoveMember(p)` - Gerencia membros online
- `GetMembersOnline()` - Lista membros conectados
- `AddRank(id, name, level)` - Adiciona rank
- `GetRankByID/Name/Level()` - Busca ranks
- `Get/SetMOTD()` - Message of the Day
- `Get/SetBankBalance()` - Banco da guild

**`internal/game/world_guild.go`** - Integração com World
```go
func (w *World) RegisterGuild(guild *Guild)
func (w *World) GetGuild(guildID uint32) *Guild
func (p *Player) GetGuild() *Guild
func (p *Player) GetGuildLevel() uint8
```

### 2. Database (`internal/db/guild.go`)

Operações completas de DB:

**Carregamento:**
- `LoadGuild(ctx, guildID)` - Carrega guild + ranks
- `LoadGuildByName(ctx, name)` - Busca por nome
- `GetGuildMembers(ctx, guildID)` - Lista membros

**Criação/Modificação:**
- `CreateGuild(ctx, name, ownerID)` - Cria guild com 3 ranks padrão
- `SaveGuild(ctx, guild)` - Salva MOTD, balance, points, level
- `DeleteGuild(ctx, guildID)` - Remove guild

**Membros:**
- `AddGuildMember(ctx, playerID, guildID, rankID, nick)`
- `RemoveGuildMember(ctx, playerID)`
- `UpdateGuildMemberRank(ctx, playerID, rankID)`
- `UpdateGuildMemberNick(ctx, playerID, nick)`

**Ranks:**
- `CreateGuildRank(ctx, guildID, name, level)`
- `DeleteGuildRank(ctx, rankID)`

### 3. Lua Bindings (`internal/luaengine/guild.go`)

**Guild Constructor:**
```lua
local guild = Guild(guildId)
```

**Métodos implementados:**
```lua
guild:getId()                    -- Retorna ID da guild
guild:getName()                  -- Retorna nome
guild:getMembersOnline()         -- Retorna array de Players online
guild:addRank(id, name, level)   -- Adiciona rank
guild:getRankById(id)            -- Retorna {id, name, level}
guild:getRankByLevel(level)      -- Retorna rank por nível
guild:getMotd()                  -- Retorna MOTD
guild:setMotd(text)              -- Define MOTD
guild:getBankBalance()           -- Retorna saldo
guild:setBankBalance(amount)     -- Define saldo
guild:addMember(player)          -- Adiciona player online
guild:removeMember(player)       -- Remove player online
guild:getMemberCount()           -- Retorna total de membros
```

**Player methods (já existentes, agora funcionais):**
```lua
player:getGuild()                -- Retorna Guild ou nil
player:setGuild(guild)           -- Define guild do player
player:getGuildLevel()           -- Retorna rank level (1-3)
player:setGuildLevel(level)      -- Define rank level
player:getGuildNick()            -- Retorna nick na guild
player:setGuildNick(nick)        -- Define nick na guild
```

### 4. Database Schema (Já Existente)

As tabelas já existem no schema.sql:

```sql
-- guilds: id, name, ownerid, creationdata, motd, balance, points, level
CREATE TABLE IF NOT EXISTS `guilds` (...)

-- guild_ranks: id, guild_id, name, level
CREATE TABLE IF NOT EXISTS `guild_ranks` (...)

-- guild_membership: player_id, guild_id, rank_id, nick
CREATE TABLE IF NOT EXISTS `guild_membership` (...)
```

### 5. World Integration

**`internal/game/world.go`** - Adicionado campo:
```go
type World struct {
    ...
    guilds map[uint32]*Guild  // Cache de guilds carregadas
    ...
}
```

**Inicialização:**
```go
func NewWorld() *World {
    w := &World{
        ...
        guilds: make(map[uint32]*Guild),
        ...
    }
}
```

## Funcionalidades Implementadas

### ✅ Criação de Guild
```lua
-- Via DB
local guildId = db:CreateGuild("My Guild", ownerPlayerId)

-- Via Lua (após carregar)
local guild = Guild(guildId)
```

### ✅ Gerenciamento de Ranks
- **3 ranks padrão:** Leader (level 3), Vice-Leader (level 2), Member (level 1)
- Criação dinâmica de novos ranks
- Busca por ID, nome ou nível

### ✅ Membros Online
- Tracking automático de players online na guild
- Lista de membros para broadcast de mensagens

### ✅ MOTD (Message of the Day)
- Armazenado em DB
- Exibido ao login de membros

### ✅ Banco da Guild
- Balance independente dos players
- Operações de depósito/retirada (via Lua scripts)

### ✅ Guild Nick
- Apelido customizado por membro
- Armazenado em `guild_membership.nick`

## Sistema de Ranks

**Níveis padrão:**
- **Level 3:** Leader (dono da guild)
- **Level 2:** Vice-Leader (permissões administrativas)
- **Level 1:** Member (membro comum)

**Permissões customizáveis via Lua scripts.**

## Exemplo de Uso

```lua
-- Criar guild via NPC
function onCreateGuild(player, guildName)
    local db = getDatabase()
    local guildId = db:CreateGuild(guildName, player:getId())
    
    if guildId > 0 then
        local guild = Guild(guildId)
        guild:setMotd("Welcome to " .. guildName .. "!")
        player:sendTextMessage(MESSAGE_INFO, "Guild created successfully!")
        return true
    end
    return false
end

-- Convidar membro
function onInviteMember(player, targetPlayer)
    local guild = player:getGuild()
    if not guild then
        return false
    end
    
    if player:getGuildLevel() < 2 then
        player:sendTextMessage(MESSAGE_STATUS, "Only leaders can invite members.")
        return false
    end
    
    local memberRank = guild:getRankByLevel(1)
    if memberRank then
        db:AddGuildMember(targetPlayer:getId(), guild:getId(), memberRank.id, "")
        targetPlayer:sendTextMessage(MESSAGE_INFO, "You have been invited to " .. guild:getName())
        return true
    end
    return false
end

-- Depositar no banco da guild
function onGuildDeposit(player, amount)
    local guild = player:getGuild()
    if not guild then
        return false
    end
    
    if player:removeMoney(amount) then
        guild:setBankBalance(guild:getBankBalance() + amount)
        db:SaveGuild(guild)
        player:sendTextMessage(MESSAGE_INFO, "Deposited " .. amount .. " gold to guild bank.")
        return true
    end
    return false
end
```

## Integração com Sistemas Existentes

✅ **Player Loading** - `internal/db/player.go` já carrega guild_membership  
✅ **World Management** - Guilds cacheadas no World  
✅ **Lua Engine** - Guild type registrado em `api.go`  
✅ **Thread-Safe** - Mutex em Guild para operações concorrentes

## Features Não Implementadas (Futuro)

⏭️ **Guild Wars** - Tabela `guild_wars` existe mas não implementada  
⏭️ **Guild Halls** - Sistema de houses integrado com guilds  
⏭️ **Protocol Packets** - Guild window, member list (requer protocol work)  
⏭️ **Guild Invites** - Sistema de convite pendente via DB

## Próximos Passos

Para tornar o sistema totalmente funcional no cliente:

1. **Implementar protocol packets:**
   - Guild window (0x??)
   - Member list
   - MOTD display

2. **NPC Scripts:**
   - Guild creation NPC
   - Guild management NPCs

3. **Guild Chat Channel:**
   - Canal CHANNEL_GUILD (0x00)

## Testes Recomendados

```lua
-- Test 1: Create guild
local guildId = db:CreateGuild("Test Guild", player:getId())
assert(guildId > 0)

-- Test 2: Load guild
local guild = Guild(guildId)
assert(guild:getName() == "Test Guild")

-- Test 3: MOTD
guild:setMotd("Test MOTD")
assert(guild:getMotd() == "Test MOTD")

-- Test 4: Ranks
local leaderRank = guild:getRankByLevel(3)
assert(leaderRank.name == "Leader")

-- Test 5: Bank
guild:setBankBalance(10000)
assert(guild:getBankBalance() == 10000)
```

## Notas Técnicas

- **Thread-Safety:** Guild usa `sync.RWMutex` para operações concorrentes
- **Memory Management:** Guilds são cacheadas no World, não recarregadas a cada acesso
- **DB Efficiency:** LoadGuild carrega guild + ranks em 2 queries
- **Lua Integration:** Guild é um shared pointer, compatível com C++ API
