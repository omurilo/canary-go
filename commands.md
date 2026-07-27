# Commands Reference

## Native Go Commands

Todos os comandos nativos exigem acesso **GM** (`gamemaster`).

| Command | Usage | Description |
|---------|-------|-------------|
| `/pos` | `/pos` | Mostra sua posição atual |
| `/up` | `/up` | Sobe um andar |
| `/down` | `/down` | Desce um andar |
| `/goto` | `/goto x y z` | Teleporta para coordenadas |
| `/go` | `/go x y z` | Aliases para `/goto` |
| `/town` | `/town <name>` | Teleporta para uma town |
| `/i` | `/i <id> [count]` | Cria um item |
| `/create` | `/create <id> [count]` | Alias para `/i` |
| `/addskill` | `/addskill <type> [n]` | Aumenta skill (fist/club/sword/axe/dist/shield/fish/ml) |
| `/save` | `/save` | Salva todos os players online |
| `/b` | `/b <text>` | Broadcast mensagem |
| `/broadcast` | `/broadcast <text>` | Alias para `/b` |
| `/outfit` | `/outfit` | Abre a janela de outfits |
| `/addexp` | `/addexp <amount>` ou `/addexp <player>, <amount>` | Adiciona experiência |
| `/addmoney` | `/addmoney <amount>` | Adiciona gold ao bank |
| `/level` | `/level [n]` | Mostra ou define level |
| `/health` | `/health [hp]` | Cura ou define HP |
| `/mana` | `/mana [mp]` | Cura ou define MP |
| `/speed` | `/speed <value>` | Define velocidade |
| `/online` | `/online` | Lista players online |
| `/info` | `/info` | Mostra informações do player |
| `/skull` | `/skull <0-5>` | Define skull (0=none 1=yellow 2=green 3=white 4=red 5=black) |
| `/sex` | `/sex` | Alterna gênero |
| `/commands` | `/commands` | Lista comandos disponíveis |

## Lua TalkActions — GM

| Command | Usage | Description |
|---------|-------|-------------|
| `/bless` | `/bless [player]` | Bless player |
| `/clean` | `/clean` | Clean tiles |
| `/ghost` | `/ghost` | Toggle ghost mode |
| `/goto` | `/goto x y z` | Teleport |
| `/info` | `/info [player]` | Player info |
| `/kick` | `/kick <player>` | Kick player |
| `/looktype` | `/looktype <id>` | Change looktype |
| `/pos` | `/pos` | Show position |
| `/teleport` | `/teleport x y z` | Teleport |
| `/town` | `/town <name>` | Teleport to town |
| `/ban` | `/ban <player>` | Ban player |
| `/unban` | `/unban <player>` | Unban player |
| `/mc` | `/mc` | Magic effects test |
| `/spy` | `/spy <player>` | Spy on player |
| `/setlight` | `/setlight <level> <color>` | Set light |
| `/listplayers` | `/listplayers` | List online |
| `/goldrank` | `/goldrank` | Gold rank |
| `/getlook` | `/getlook` | Get look info |
| `/namelock` | `/namelock` | Name lock |
| `/distanceeffect` | `/distanceeffect` | Distance effect |
| `/countmonsters` | `/countmonsters` | Count monsters |
| `/effect` | `/effect <id>` | Show magic effect |

## Lua TalkActions — God

| Command | Usage | Description |
|---------|-------|-------------|
| `/i` | `/i <id> [count]` | Create item |
| `/r` | `/r <id>` | Reload |
| `/s` | `/s <player>` | Summon monster |
| `/n` | `/n <name>` | Create NPC |
| `/m` | `/m <name>` | Monster info |
| `/a` | `/a <text>` | Announce |
| `/c` | `/c <name>` | Summon creature |
| `/t` | `/t <text>` | Send message |
| `/reload` | `/reload <type>` | Reload systems |
| `/save` | `/save` | Save all |
| `/raid` | `/raid <name>` | Start raid |
| `/spawn` | `/spawn` | Spawn management |
| `/owner` | `/owner [player]` | Set house owner |
| `/vip` | `/vip [player]` | Toggle VIP |
| `/ban` | `/ban <player>` | Ban |
| `/ipban` | `/ipban <player>` | IP ban |
| `/inbox` | `/inbox` | Open inbox |
| `/raid` | `/raid <name>` | Start raid |
| `/simraid` | `/simraid <name>` | Simulate raid |
| `/listraid` | `/listraid` | List raids |
| `/proficiency` | `/proficiency <exp>` | Add weapon proficiency exp |
| `/addbadge` | `/addbadge <id>` | Add badge |
| `/zones` | `/zones` | Zone info |
| `/tile` | `/tile` | Tile info |
| `/iteminfo` | `/iteminfo` | Item info |
| `/door` | `/door` | Door info |
| `/addtitle` | `/addtitle <id>` | Add title |
| `/addskill` | `/addskill <type> <level>` | Set skill level |
| `/addmoney` | `/addmoney <amount>` | Add gold |
| `/addmount` | `/addmount <id>` | Add mount |
| `/addaddon` | `/addaddon <looktype> <addon>` | Add outfit addon |
| `/addachievement` | `/addachievement <id>` | Add achievement |
| `/addbosskill` | `/addbosskill <raceId>` | Add boss kill |
| `/addcharms` | `/addcharms <id>` | Add charm |
| `/charmexpansion` | `/charmexpansion` | Toggle charm expansion |
| `/addreward` | `/addreward` | Add reward |
| `/addtutor` | `/addtutor <player>` | Add tutor |
| `/removetutor` | `/removetutor <player>` | Remove tutor |
| `/hireling` | `/hireling` | Hireling management |
| `/closeserver` | `/closeserver` | Close server |
| `/openserver` | `/openserver` | Open server |
| `/getkv` | `/getkv <key>` | Get KV value |
| `/setkv` | `/setkv <key> <value>` | Set KV value |
| `/flags` | `/flags` | Flag management |
| `/testicon` | `/testicon` | Test icon |
| `/protocolprobe` | `/protocolprobe` | Protocol probe |
| `/probeopcode` | `/probeopcode` | Probe opcode |
| `/testlog` | `/testlog` | Test log |
| `/testmessage` | `/testmessage <id>` | Test message |
| `/sound` | `/sound <type> <id>` | Sound test |

## Lua TalkActions — Player

| Command | Usage | Description |
|---------|-------|-------------|
| `!commands` | `!commands` | List commands |
| `!online` | `!online` | List online players |
| `!time` | `!time` | Server time |
| `!balance` | `!balance` | Bank balance |
| `!deposit` | `!deposit <amount>` | Deposit gold |
| `!withdraw` | `!withdraw <amount>` | Withdraw gold |
| `!transfer` | `!transfer <player>, <amount>` | Transfer gold |
| `!buyhouse` | `!buyhouse` | Buy house |
| `!sellhouse` | `!sellhouse` | Sell house |
| `!leavehouse` | `!leavehouse` | Leave house |
| `!bless` | `!bless` | Bless |
| `!flask` | `!flask` | Get flask |
| `!reward` | `!reward` | Claim reward |
| `!refill` | `!refill` | Refill HP/MP |
| `!serverinfo` | `!serverinfo` | Server info |
| `!emote` | `!emote` | Emote window |
| `!livestream` | `!livestream` | Livestream |
| `!aol` | `!aol` | AOL |
| `!position` | `!position` | Position info (tutor+) |
