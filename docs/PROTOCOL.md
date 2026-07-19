# Canary wire protocol — implementation reference

Condensed from reverse-engineering `src/server/network/**` of the C++ server.
All integers **little-endian**. Strings = `u16 length` + raw bytes (no NUL).
`Position` = `u16 x, u16 y, u8 z`.

## Constants
- `HEADER_LENGTH = 2` (outer length field)
- `CHECKSUM_LENGTH = 4`
- `XTEA_MULTIPLE = 8`
- `NETWORKMESSAGE_MAXSIZE = 65500`, `INPUTMESSAGE_MAXSIZE = 4096`
- `INITIAL_BUFFER_POSITION = 8` (outbound staging offset; inbound body starts at offset 2)
- `CLIENT_VERSION = 1525` (Tibia 13.x), `SERVER_BEAT = 50`
- Viewport 18×14 (VIEWPORT_X=8, VIEWPORT_Y=6)

## Checksum
- Methods: NONE=0, ADLER32=1, SEQUENCE=2. 4 bytes, right after the 2-byte length.
- Adler32: standard, MOD 65521, over the encrypted region.
- Sequence: monotonic u32 counter (separate in/out), reset at 0x7FFFFFFF; high bit (1<<31) OR'd in signals raw-DEFLATE compression.
- Login uses ADLER32; modern game client (OS <= OTCLIENT_MAC=12) uses SEQUENCE.
- First inbound packet (no protocol yet): server tries adler32 over bytes after checksum; if mismatch, treats as no-checksum. Then reads 1 protocol-id byte (0x01 = login).

## XTEA (canonical TFS form)
- Key: 4×u32 (LE). Block: 8 bytes = 2 u32 (LE). 32 rounds. ECB.
- Encrypt: sum=0; per round: `v0 += (((v1<<4)^(v1>>5))+v1) ^ (sum+k[sum&3]); sum+=0x9E3779B9; v1 += (((v0<<4)^(v0>>5))+v0) ^ (sum+k[(sum>>11)&3])`
- Decrypt: sum=0xC6EF3720; per round: `v1 -= (((v0<<4)^(v0>>5))+v0) ^ (sum+k[(sum>>11)&3]); sum-=0x9E3779B9; v0 -= (((v1<<4)^(v1>>5))+v1) ^ (sum+k[sum&3])`
- (0x9E3779B9 == -0x61C88647 mod 2^32; equivalent to the server's negated-delta form.)

## RSA
- 1024-bit (128-byte block), e=65537, **raw** (no padding): m = c^d mod n on the 128-byte big-endian block.
- First login packet of both login & game protocols carries exactly one 128-byte RSA block.
- After decrypt, first byte MUST be 0x00; then 4×u32 XTEA key follow.

## Payload layouts (after checksum, XTEA region)
- ModernPaddingByte (CurrentModern): plaintext[0]=paddingCount, body, then 0x33 filler → multiple of 8. On recv: read padByte, real len = blockBytes - padByte.
- LegacyInnerLength (LegacyClassic): prepend u16 innerLength.

## Outbound order (encrypted)
1. prepend padding-count byte (modern) or u16 inner-length (legacy); pad body to *8 with 0x33
2. XTEA encrypt
3. prepend checksum (adler32 over encrypted region) or sequence u32
4. prepend outer length: modern = u16((len-4)/8) block count; legacy/raw = u16(rawLen)

Plaintext (no encryption, e.g. login challenge): just prepend outer length.

## Transport profiles
| Profile | outerLen | payload | checksum | compress |
|---|---|---|---|---|
| RawClientFirst | raw | none | none | no |
| CurrentModern (13.x) | blockCount | modernPad | sequence | yes(zlib raw -15) |
| LegacyClassic / LegacyRawWithLoginHeader (8.6) | raw | innerLen | adler32 | no |

decodeBodySize: modern = header*8 + 4; raw = header.

## LOGIN protocol (port 7171), client-first
Inbound first packet: `[u16 rawLen][u32 adler][u8 0x01 protoId][u16 os][u16 version]` then version-specific skip (Current: skip 17 = u32 clientver + 12 sig + 1 preview), then 128-byte RSA block:
`[0x00][4×u32 xteaKey][str accountDescriptor(email)][str password]`. Then set XTEA + ADLER32.
- sessionKey = `accountDescriptor + "\n" + password`.
- Auth: Account.load (by email modern / name old) + authenticate (argon2 PHC, fallback SHA1 hex).

Outbound (Current transport = block-count + modern-pad + adler32):
- Error: `0x0B [str message]`, then close.
- MOTD (if set): `0x14 [str "<num>\n<text>"]`.
- Session key (Current/1100): `0x28 [str sessionKey]`.
- Char list `0x64`:
  `0x01`(#worlds) `0x00`(worldid) `[str serverName][str worldIp(string)][u16 gamePort][0x00]`
  `[u8 charCount]` per char: `[0x00 worldid][str name]`
  `[u8 premDaysTruncated][u8 isPremiumNow][u32 premiumLastDay]`
  then close.

## GAME protocol (port 7172), server-first (challenge)
On connect server sends challenge (plaintext): inner `0x1F [u32 timestamp][u8 random][0x71]`.
Inbound login packet:
`[u16 os][u16 version][u32 clientVersion][str clientVersionString][str assetHash][u8 previewState]`
128-byte RSA block: `[0x00][4×u32 xteaKey][u8 isGameMaster]` then (SessionKey layout) `[str sessionKey][str characterName]`.
Then XTEA-plaintext continuation: `[u32 timeStamp==challenge][u8 rand==challenge]` then OTCv8 probe `[u16 len; if 5 && "OTCv8": u16 v]`.
- sessionKey split on '\n' → account, password. clientVersion must == 1525.
- On success: dispatcher `login(charName, accountId, os)`.

### Enter-world sequence (server → client), in order:
1. `0x17` Login/SelfAppear: `u32 playerID, u16 SERVER_BEAT(50), double speedA,B,C, u8 canChangePvp(0), u8 expertMode(0), str storeUrl, u16 storeCoinPkg, u8 exivaButton`
2. `0x1A` AllowBugReport: `u8 0`
3. `0xEF` TibiaTime: `u8 hour, u8 minute`
4. `0x0A` PendingStateEntered (empty)
5. `0x0F` EnterWorld (empty)
6. `0x64` FullMapDescription: `Position(playerPos)` + map (see below)
7. `0x83` MagicEffect login teleport at pos
8. `0x85` DisableLoginMusic: `u8 1, u16 0` (skip for OTClient)
9. `0x78/0x79` inventory per slot
10. `0xA0` PlayerStats (see below)
11. `0xA1` PlayerSkills
12. side systems (bless/premium/prices/prey/forge) — optional
13. `0x82` WorldLight: `u8 level, u8 color`
14. `0x8D` CreatureLight: `u32 creatureID, u8 level, u8 color`
15. `0x9F` BasicData
16. GameNews/Icons etc — optional

### 0xA0 PlayerStats (13.x)
`u32 hp, u32 maxHp, u32 freeCap, u64 exp, u16 level, u16 levelPercent(*100 cap 10000), u16 baseXp, u16 grindXp, u16 xpBoost, u16 staminaMult, u32 mana, u32 maxMana, u8 soul, u16 staminaMin, u16 baseSpeed, u16 regenSecs, u16 offlineTrainMin, u16 xpBoostTime, u8 canBuyXpBoost(1), u32 manaShield, u32 maxManaShield`

### 0xA1 PlayerSkills (13.x): sequence of skills each `u16 level, u16 baseLevel, u16 loyaltyBonus, u16 percent(*100)` — order per client. (Minimal impl sends fist..fishing + special.)

### 0x9F BasicData
`u8 isPremium, u32 premiumExpireTs, u8 vocationClientId, u8 hasReachedMain, u16 spellCount, [u16 spellId]*, u8 magicShieldActive`

### Map description (0x64)
`sendMapDescription`: `0x64 Position(pos)` + `GetMapDescription(px-8, py-6, pz, width=18, height=14)`.
Floors: if z>7: startz=z-2..min(15,z+2) step +1; else startz=7..0 step -1.
For each floor `GetFloorDescription(x,y,nz, w,h, offset=z-nz, &skip)`:
  loop nx 0..w, ny 0..h; tile at (x+nx+offset, y+ny+offset, z):
   - tile exists: if skip>=0 emit `u8 skip, u8 0xFF`; skip=0; then GetTileDescription
   - empty & skip==0xFE: emit `0xFF 0xFF`, skip=-1
   - empty else: skip++
After all floors: if skip>=0 emit `u8 skip, u8 0xFF`.
GetTileDescription: ground item(AddItem), top items, creatures, down items; max 10 things.
AddItem (13.x): `u16 itemId` [+u8 count if stackable] [+u8 fluid if splash] ...
AddCreature (new): `u16 0x61, u32 removedKnownId, u32 creatureID, u8 creatureType, str name, u8 healthPct, u8 direction, Outfit, u8 lightLevel, u8 lightColor, u16 stepSpeed, u8 iconCount(0), u8 skull, u8 party, u8 guildEmblem(new only), u8 creatureType, [u8 vocClientId if PLAYER], u8 speechBubble, u8 0xFF, u8 0x00, u8 walkthrough`
  known: `u16 0x62, u32 creatureID`
Outfit: `u16 lookType; if !=0 {u8 head,body,legs,feet,addons} else {u16 lookTypeEx}; u16 lookMount [+u8 mHead,mBody,mLegs,mFeet if mount!=0]`

### Inbound game opcodes (dispatch)
0x14 logout, 0x1D pong, 0x1E ping, 0x64 autowalk, 0x65 N,0x66 E,0x67 S,0x68 W, 0x69 stop, 0x6A NE,0x6B SE,0x6C SW,0x6D NW, 0x6F turnN,0x70 turnE,0x71 turnS,0x72 turnW, 0x96 say, 0x97 requestChannels, 0x8C lookAt, 0xA0 fightModes, 0xA1 attack, 0x32 extendedOpcode.

## DB (PostgreSQL)
Key tables: accounts, players, player_items, player_storage, global_storage, player_spells, guilds, guild_membership, towns, players_online, account_sessions.
accounts: id serial pk, name unique, password, email, premdays, lastday, type, coins, creation...
players: id serial pk, name unique, account_id, level, vocation, health/max, mana/max, experience, look*, posx/posy/posz, town_id, sex, skills skill_*(+_tries), cap, balance, conditions bytea...
Auth: argon2 PHC (fallback SHA1 hex). Player items/conditions are OTBR binary blobs (PropStream).
