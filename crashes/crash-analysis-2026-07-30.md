# Análise de Crash — Tibia Client v15.25.0 → Canary-Go

## Metadados

| Campo | Valor |
|---|---|
| **Data do crash** | 30/07/2026 ~23:40 UTC (20:40 UTC-3) |
| **Personagem** | Gm Test |
| **Servidor** | Canary-Go (127.0.0.1:7172) |
| **Cliente** | Tibia Client v15.25.0 (EngineID "0" — OTClient) |
| **Sistema** | Windows 11 Version 25H2 |
| **BattlEye** | v1.249 |
| **Dump** | Mini DuMP 1.2MB, x86_64, 59 threads |
| **Exceção** | `0xE06D7363` (Microsoft C++ Exception) em `KERNELBASE!RaiseException` |

---

## Timeline do Crash

```
20:40:13.881  BattlEye inicializado
20:40:13.953  Client → GameServer: "Gm Test" conectando (127.0.0.1:7172)
20:40:13.971  Conectado ao gameserver "Canary-Go"
20:40:13.973  News query failed (HTTP 127.0.0.1:8088 — servidor não responde)
20:40:13.995  ⚠️ ERROR: "field has more than one zero id appearance"
20:40:13.995  Log do pacote problemático (1672 bytes, inicia com 0x17=SelfAppear)
20:40:14.172  Qt: "caught an exception thrown from an event handler"
20:40:17.707  Cliente tenta se recuperar (recarrega UI)
20:40:18.846  "QObject: Cannot create children for a parent that is in a different thread"
~23:40:14      Crash dump gerado
```

---

## Cadeia de Eventos (Protocolo)

```
1. Client connecta ao gameserver (porta 7172)
2. Server envia enterWorld (opcodes 0x17+0x1A+0xEF+0x0A+0x0F+0x64+...)
3. Server envia coin balance (0xDF/0xF2)
4. OTClient recebe coin balance → abre store automaticamente
5. Client → Server: C_OpenStore (0xFA)
6. Server → Client: S_OpenStore (0xFB) — lista de categorias
7. Client → Server: C_RequestStoreOffers (0xFB)
8. Server → Client: S_StoreOffers (0xFC) — ofertas da primeira categoria
9. ⚠️ OTClient falha ao processar S_StoreOffers → "field has more than one zero id appearance"
10. Exceção C++ propaga pelo Qt → crash
```

---

## Causa Raiz

### Erro: "field has more than one zero id appearance"

O erro ocorre no OTClient ao processar o pacote `S_StoreOffers` (0xFC). O cliente espera que cada oferta tenha um **ID único e não-zero**. Quando encontra múltiplas ofertas com `ID=0`, o parser lança o erro e o cliente trava.

### Onde o pacote é montado

**Arquivo:** `data/libs/gamestore/senders.lua`
**Função:** `sendShowStoreOffers()` (linha 92) — envia `S_StoreOffers` (0xFC)
**Detalhe crítico — linha 197:**
```lua
msg:addU32(off.id)
```
Se `off.id` for `nil` ou inválido, `addU32` escreve `0` no pacote.

### Onde os IDs são atribuídos

**Arquivo:** `data/modules/scripts/gamestore/gamestore.lua`
**Função:** Atribuição inline (linhas 63-87)
```lua
local runningId = 45000
for k, category in ipairs(GameStore.Categories) do
    if category.offers then
        for m, offer in ipairs(category.offers) do
            if not offer.id then
                offer.id = runningId
                runningId = runningId + 1
            end
```

Esta atribuição acontece uma vez no carregamento do módulo. ID dinâmico começa em 45000.

### Observações sobre o padrão coinType

**Arquivo:** `data/libs/gamestore/store.lua`
**Função:** `normalizeOffer()` (linha 197)

```lua
-- store.lua:200-203
if offer.coinType == nil then
    offer.coinType = GameStore.CoinType.Transferable  -- DEFAULT é Transferable (1)
```

Em `constants.lua`:
```lua
GameStore.CoinType = {
    Coin = 0,
    Transferable = 1,
}
```

O **default de coinType é Transferable (1)**, não Coin (0). TODO o catálogo usa Transferable como padrão. Isso é incomum — no Tibia original, a maioria das ofertas usa moedas normais.

### Observações sobre ofertas sem id

Algumas ofertas no catálogo **não têm campo `id`**:
- `consumables_blessings.lua`: "Death Redemption" e "Twist of Fate (5 count)" não têm `id`
- `cosmetics_outfits.lua`: NENHUMA oferta tem `id` (recebem IDs dinâmicos: 45000+)
- Outros catálogos seguem o mesmo padrão

Essas ofertas DEPENDEM da atribuição dinâmica em `gamestore.lua`. Se houver qualquer erro silencioso na atribuição, elas seriam enviadas com `ID=0`.

---

## Impacto

O client trava **sempre que o jogador conecta ao servidor**, pois:
1. O OTClient detecta que o jogador tem coins (via balance packet)
2. OTClient abre a store automaticamente
3. OTClient requisita ofertas
4. Servidor responde com ofertas → parsing falha → crash

**Impossível jogar** enquanto o crash não for corrigido.

---

## Correção Recomendada

### 1. Log de depuração no carregamento do catálogo

Adicionar verificação em `gamestore.lua` após a atribuição de IDs para confirmar que todas as ofertas têm IDs únicos:

```lua
local seenIds = {}
for k, category in ipairs(GameStore.Categories) do
    if category.offers then
        for m, offer in ipairs(category.offers) do
            if not offer.id or offer.id == 0 then
                logger.warn("[store] offer without valid id: {} ({})", offer.name, category.name)
            elseif seenIds[offer.id] then
                logger.warn("[store] duplicate offer id: {} ({})", offer.id, offer.name)
            end
            seenIds[offer.id] = true
        end
    end
end
```

### 2. Validar `addU32` no NetworkMessage

Garantir que `addU32(nil)` não escreva 0 silenciosamente — deve lançar um erro visível para depuração.

### 3. Verificar formato do S_StoreOffers para protocolo 1525

Confirmar que os campos extras do `sendShowStoreOffers` (linhas 114-118):
```lua
msg:addU32(redirectId or 0)
msg:addByte(0) -- Window Type
msg:addByte(0) -- Collections Size
msg:addU16(0)  -- Collection Name
```
Estão na ordem e formato corretos para o OTClient versão 1525.

---

## Referências

### Parsing (servidor → client)

| Opcode | Constante | Arquivo | Função |
|---|---|---|---|
| `0xFB` | `S_OpenStore` | `senders.lua:20` | `openStore()` |
| `0xFC` | `S_StoreOffers` | `senders.lua:92` | `sendShowStoreOffers()` |
| `0xFC` | `S_StoreOffers` | `senders.lua:556` | `sendHomePage()` |

### Request (client → servidor)

| Opcode | Constante | Arquivo | Função |
|---|---|---|---|
| `0xFA` | `C_OpenStore` | `parsers.lua:139` | `parseOpenStore()` |
| `0xFB` | `C_RequestStoreOffers` | `parsers.lua:148` | `parseRequestStoreOffers()` |
| `0xFC` | `C_BuyStoreOffer` | `parsers.lua:246` | `parseBuyStoreOffer()` |

### Inicialização do módulo

| Arquivo | Descrição |
|---|---|
| `data/modules/scripts/gamestore/init.lua` | Loader do módulo |
| `data/modules/scripts/gamestore/gamestore.lua` | Inicialização, atribuição de IDs |
| `data/modules/scripts/gamestore/catalog_loader.lua` | Validação e registro de ofertas |
| `data/modules/scripts/gamestore/catalog/init.lua` | Lista de módulos do catálogo (25 módulos) |
| `data/libs/gamestore/constants.lua` | Constantes do protocolo |
| `data/libs/gamestore/store.lua` | Helpers (getOfferById, normalizeOffer, etc.) |

### Minidump

| Item | Local |
|---|---|
| **Dump original** | `/mnt/c/Users/Pichau/Downloads/tibia-client/log/crashdump/` (vazio) |
| **Dump analisado** | Renomeado para `418a4a4f-dffb-47b5-a228-08b2839786f8.dmp` |
| **Client log** | `/mnt/c/Users/Pichau/Downloads/tibia-client/log/client.log` (2542 linhas) |
| **Config cliente** | `/mnt/c/Users/Pichau/Downloads/tibia-client/conf/config.ini` |
| **Crash report JSON** | `crash-2026-07-30-gm-test.json` (neste diretório) |

---

## Arquivos Afetados

```
data/libs/gamestore/senders.lua           # MONTA os pacotes S_OpenStore e S_StoreOffers
data/libs/gamestore/parsers.lua            # Processa requisições do cliente
data/libs/gamestore/store.lua              # normalizeOffer (coinType default Transferable)
data/libs/gamestore/constants.lua          # Constantes CoinType, OfferTypes, etc.
data/modules/scripts/gamestore/gamestore.lua  # Atribuição de IDs dinâmicos
data/modules/scripts/gamestore/catalog_loader.lua  # Validação de ofertas
data/modules/scripts/gamestore/catalog/    # Definições do catálogo (25 arquivos)
internal/protocol/game.go                  # dispatchStore (ponte Go→Lua)
internal/luaengine/store.go                # DispatchStorePacket
```
