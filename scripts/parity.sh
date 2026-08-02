#!/usr/bin/env bash
# Measures how far the Go port is from the C++ server, by counting things that
# can be counted rather than by reading a document.
#
# Written because the previous parity write-up went stale without anyone
# noticing: three of its headline numbers were wrong by the time they were acted
# on, and one of them (the enum gap) came from a tool that was itself broken.
# Anything asserted here is re-derived on every run.
#
# Usage: scripts/parity.sh [--md]
set -uo pipefail

GO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CPP_ROOT="$(cd "$GO_ROOT/.." && pwd)"
SRC="$CPP_ROOT/src"

[[ -d "$SRC" ]] || {
	echo "C++ source not found at $SRC" >&2
	exit 1
}

MD=0
[[ "${1:-}" == "--md" ]] && MD=1

row() { # name cpp go note
	if ((MD)); then
		printf '| %s | %s | %s | %s |\n' "$1" "$2" "$3" "$4"
	else printf '  %-30s C++ %-8s Go %-8s %s\n' "$1" "$2" "$3" "$4"; fi
}
head_() {
	if ((MD)); then
		printf '\n### %s\n\n| what | C++ | Go | note |\n|---|---|---|---|\n' "$1"
	else printf '\n== %s ==\n' "$1"; fi
}

# ---- size -------------------------------------------------------------------
head_ "Size"
cpp_files=$(find "$SRC" -name '*.cpp' -o -name '*.hpp' | wc -l | tr -d ' ')
go_files=$(find "$GO_ROOT/internal" "$GO_ROOT/cmd" -name '*.go' ! -name '*_test.go' | wc -l | tr -d ' ')
cpp_loc=$(find "$SRC" -name '*.cpp' -o -name '*.hpp' | xargs cat 2>/dev/null | wc -l | tr -d ' ')
go_loc=$(find "$GO_ROOT/internal" "$GO_ROOT/cmd" -name '*.go' ! -name '*_test.go' | xargs cat 2>/dev/null | wc -l | tr -d ' ')
go_test_loc=$(find "$GO_ROOT/internal" -name '*_test.go' | xargs cat 2>/dev/null | wc -l | tr -d ' ')

# MUDANÇA 1: Adicionada proteção contra divisão por zero
row "source files" "$cpp_files" "$go_files" "$((go_files * 100 / (cpp_files > 0 ? cpp_files : 1)))%"
row "lines of code" "$cpp_loc" "$go_loc" "$((go_loc * 100 / (cpp_loc > 0 ? cpp_loc : 1)))%"
row "test lines" "-" "$go_test_loc" ""

# ---- inbound opcodes --------------------------------------------------------
# Both sides dispatch from one switch. Counting the case labels that actually
# reach a handler is the only honest measure: a handler that exists but is never
# dispatched is dead code, and this port has had plenty.
head_ "Inbound opcodes (client -> server)"
cpp_in=$(sed -n '/void ProtocolGame::parsePacket/,/^}/p' \
	"$SRC/server/network/protocol/protocolgame.cpp" 2>/dev/null |
	grep -cE '^\s*case 0x[0-9A-Fa-f]+:')
go_in=$(sed -n '/func (g \*GameProtocol) OnPacket/,/^}/p' \
	"$GO_ROOT/internal/protocol/game.go" | grep -cE '^\s*case (0x[0-9A-Fa-f]+|in[A-Za-z]+)')
go_in_handlers=$(grep -rhoE 'func \(g \*GameProtocol\) parse[A-Za-z]+' \
	"$GO_ROOT/internal/protocol/" | sort -u | wc -l | tr -d ' ')
go_in_called=$(grep -rhoE 'g\.parse[A-Za-z]+\(' "$GO_ROOT/internal/protocol/game.go" | sort -u | wc -l | tr -d ' ')
row "dispatched cases" "$cpp_in" "$go_in" "$((go_in * 100 / (cpp_in > 0 ? cpp_in : 1)))%"
row "parse* defined" "-" "$go_in_handlers" ""
row "parse* dispatched" "-" "$go_in_called" "$((go_in_handlers - go_in_called)) never reached"

# ---- outbound opcodes -------------------------------------------------------
head_ "Outbound opcodes (server -> client)"
cpp_out=$(grep -ohE 'msg\.addByte\(0x[0-9A-Fa-f]{2}\)' \
	"$SRC/server/network/protocol/protocolgame.cpp" | sort -u | wc -l | tr -d ' ')
# Go sends most frames through named constants (opFullMap, opContainerOpen...),
# so counting only literal AddByte(0x..) undercounts badly. Resolve the names to
# their values and union the two. Getting this wrong is how the previous
# write-up reported 53% for a surface that is much better covered.
go_out=$({
	grep -rhoE 'AddByte\(0x[0-9A-Fa-f]{2}\)' "$GO_ROOT/internal/protocol/" |
		grep -oE '0x[0-9A-Fa-f]{2}'
	grep -rhoE '\bop[A-Z][A-Za-z]*[[:space:]]*=[[:space:]]*0x[0-9A-Fa-f]{2}' "$GO_ROOT/internal/protocol/" |
		grep -oE '0x[0-9A-Fa-f]{2}'
} | tr 'a-f' 'A-F' | sort -u | wc -l | tr -d ' ')
row "distinct opcodes sent" "$cpp_out" "$go_out" "$((go_out * 100 / (cpp_out > 0 ? cpp_out : 1)))%"

# ---- Lua surface ------------------------------------------------------------
head_ "Lua API"
cpp_methods=$(grep -rhoE 'registerMethod\(L, "[A-Za-z]+", "[A-Za-z_]+"' "$SRC/lua/" | sort -u | wc -l | tr -d ' ')
go_methods=$(grep -rhoE '^\s*"[a-z][A-Za-z0-9_]*":' "$GO_ROOT/internal/luaengine/" | sort -u | wc -l | tr -d ' ')

# MUDANÇA 2: Variáveis dinâmicas no lugar de valores "hardcoded"
cpp_enums=$(grep -rhcE 'registerEnum' "$SRC/lua/functions/core/game/lua_enums.cpp" 2>/dev/null | head -1)
# O comando abaixo é uma sugestão de busca para o Go; ajuste o 'grep' conforme as declarações no seu código.
go_enums=$(grep -rhoE 'registerEnum' "$GO_ROOT/internal/luaengine/" 2>/dev/null | wc -l | tr -d ' ')

row "class methods" "$cpp_methods" "$go_methods" "names only, not behaviour"
# O uso de ':-0' garante que, se o grep não encontrar nada e as variáveis ficarem vazias, o valor padrão seja 0.
row "enum globals" "${cpp_enums:-0}" "${go_enums:-0}" "raw grep counts; the real diff is TestRegisteredEnumsMatchUpstream"

# ---- database ---------------------------------------------------------------
head_ "Database tables referenced from code"
touched=0
untouched=""
while read -r t; do
	[[ -z "$t" ]] && continue
	if grep -rqF "$t" "$GO_ROOT/internal" "$GO_ROOT/cmd" --include='*.go' 2>/dev/null; then
		touched=$((touched + 1))
	else
		untouched="$untouched $t"
	fi
done < <(grep -ohE 'CREATE TABLE IF NOT EXISTS `[a-z_]+`' "$GO_ROOT/schema/schema.sql" 2>/dev/null |
	sed 's/.*`\(.*\)`/\1/' | sort -u)

# MUDANÇA 3: Adicionado o '|| echo 0' para prevenir string vazia se o arquivo SQL não existir
total=$(grep -cE 'CREATE TABLE IF NOT EXISTS' "$GO_ROOT/schema/schema.sql" 2>/dev/null || echo 0)

row "tables in schema" "-" "$total" ""
row "referenced from Go" "-" "$touched" "$((touched * 100 / (total > 0 ? total : 1)))%"
if ((MD)); then
	printf '\nNever referenced:%s\n' "$untouched"
else printf '  never referenced:%s\n' "$untouched"; fi

# ---- subsystems -------------------------------------------------------------
# Ratios of implementation size against the C++ file that owns the same job.
# A low ratio is a hint to go and read, not a verdict.
head_ "Subsystem size ratios"
# go-path may name several files. A subsystem split across more than one Go file
# was counted from the first alone and reported far lower than it is — monster AI
# read 3% while half of it sat in monster_ai.go.
# Both sides may span several files, and BOTH lists have to cover the same scope
# or the ratio is meaningless. Counting Go's Lua bindings against the C++ core
# alone reported NPC at 131%, which is as wrong as the 9% it replaced.
#
#   cmp_pair <label> <cpp...> -- <go...>
cmp_pair() {
	local label c g f side
	label="$1"
	shift
	c=0
	g=0
	side=cpp
	for f in "$@"; do
		if [[ "$f" == "--" ]]; then
			side=go
			continue
		fi
		if [[ "$side" == "cpp" ]]; then
			c=$((c + $(cat "$SRC/$f" 2>/dev/null | wc -l | tr -d ' ')))
		else
			g=$((g + $(cat "$GO_ROOT/$f" 2>/dev/null | wc -l | tr -d ' ')))
		fi
	done
	((c == 0)) && return
	row "$label" "$c" "$g" "$((g * 100 / c))%"
}
cmp_pair "monster AI" "creatures/monsters/monster.cpp" \
	-- "internal/game/ai_engine.go" "internal/game/monster_ai.go" "internal/game/monster_think.go"
cmp_pair "npc (core)" "creatures/npcs/npc.cpp" \
	-- "internal/game/npc.go" "internal/game/npc_engine.go" "internal/game/npc_shop.go"
cmp_pair "npc (lua api)" \
	"lua/functions/creatures/npc/npc_functions.cpp" \
	"lua/functions/creatures/npc/npc_type_functions.cpp" \
	"lua/functions/creatures/npc/shop_functions.cpp" \
	-- "internal/luaengine/npc.go" "internal/luaengine/npctype.go"
cmp_pair "house" "map/house/house.cpp" -- "internal/game/house.go"
cmp_pair "spawn" "creatures/monsters/spawns/spawn_monster.cpp" -- "internal/game/spawn_engine.go"
cmp_pair "decay" "items/decay/decay.cpp" -- "internal/game/decay.go"
cmp_pair "map serialize" "io/iomapserialize.cpp" -- "internal/db/tile_store.go"

# ---- behaviour coverage -----------------------------------------------------
# Size ratios say nothing about whether a behaviour exists. This walks the C++
# methods of one class and asks whether each has a Go counterpart by name, which
# is the thing that actually has to reach 1:1.
#
# Matching is by name, so it can be fooled by a same-named function that does
# something else. It cannot be fooled by absence, which is the failure mode that
# matters here.
head_ "Behaviour coverage (methods with a Go counterpart)"

# Names that will never have a Go counterpart. Anything not listed here and not
# found counts as missing.
#
# The compute/* group is this fork's asynchronous scheduling machinery (target
# search, combat intention, follow path). Porting it literally into Go would be
# wrong — the concurrency model is different — and the behaviour it computes is
# covered by the synchronous path.
SKIP_METHODS='
Monster createMonster getMonster setID addList removeList
Npc createNpc getNpc getLowerName setSpawnNpc
getInstance
getName setName getTypeName getNameDescription setNameDescription getDescription
getType getMasterPos setMasterPos getRaceId getRace getMonsterType
isDead setDead getIdleStatus isTargetNearby israndomStepping
getIgnoreFieldDamage setIgnoreFieldDamage setFatalHoldDuration
getCriticalDamage setCriticalDamage getCriticalChance setCriticalChance
getRespawnType setSpawnMonster getForgeStack setForgeStack isForgeCreature
setForgeMonster getMonsterForgeClassification setMonsterForgeClassification
getTimeToChangeFiendish setTimeToChangeFiendish
getHazard setHazard getHazardSystemCrit setHazardSystemCrit
getHazardSystemDodge setHazardSystemDodge getHazardSystemDamageBoost
setHazardSystemDamageBoost getHazardSystemDefenseBoost setHazardSystemDefenseBoost
getSoulPit setSoulPit setImmune
requestTargetSearchCompute prepareTargetSearchCompute completeTargetSearchCompute
retryTargetSearchCompute clearTargetSearchCompute nextTargetSearchComputeGeneration
markTargetStateChanged markTargetDecisionChanged deferTargetSelection
searchTargetImmediate requestCombatIntention startPendingCombatIntention
prepareCombatIntention completeCombatIntention deferPendingCombatIntention
clearCombatIntention commitCombatIntention nextCombatIntentionGeneration
requestFollowPathCompute supersedeFollowPathCompute prepareFollowPathCompute
completeFollowPathCompute rejectFollowPathCompute discardFollowPathCompute
nextFollowPathComputeGeneration capturePathTraits captureComputeRelevance
isComputeRelevant onExecuteAsyncTasks trySchedulePostThink
cancelScheduledPostThink executePostThink queuePostThinkAfterAsync
promotePostThinkToPlayerVisibleQueue onThink_async queueMovementAiRefresh
executeMovementAiRefresh processMovementAiRefresh isPlayerVisibleForScheduling
observeVisiblePlayerForScheduling addVisiblePlayerSpectator
removeVisiblePlayerSpectator forgetTargetReference onFollowCreatureComplete
countsAsPlayerOnScreenTarget
'

behaviour_coverage() { # cpp-file  go-dir  cpp-class  [go-receiver]
	local cpp="$SRC/$1" godir="$GO_ROOT/$2" class="$3" recv="${4:-$3}"
	[[ -f "$cpp" ]] || return
	local total=0 have=0 skipped=0 name upper flat
	local missing=""
	# Collapse the newline-separated skip list to single spaces so the
	# word-boundary test below works.
	flat=" $(echo $SKIP_METHODS) "

	while read -r name; do
		if [[ "$flat" == *" $name "* ]]; then
			skipped=$((skipped + 1))
			continue
		fi
		total=$((total + 1))
		# Go convention is the same name, exported. Accept either capitalisation:
		# an unexported helper is still a counterpart.
		upper="$(printf '%s' "${name:0:1}" | tr '[:lower:]' '[:upper:]')${name:1}"
		if grep -rqE "func \([a-z]+ \*$recv\) ($name|$upper)\(" "$godir" 2>/dev/null ||
			grep -rqE "func ($name|$upper)\(" "$godir" 2>/dev/null; then
			have=$((have + 1))
		else
			missing="$missing $name"
		fi
	done < <(grep -oE "(^|[^A-Za-z_])$class::[a-zA-Z_]+\(" "$cpp" | sed "s/.*$class:://;s/(//" | sort -u)

	((total == 0)) && return
	row "$class" "$total" "$have" "$((have * 100 / total))% ($skipped skipped)"
	if [[ -n "$missing" ]]; then
		if ((MD)); then printf '\nMissing:%s\n' "$missing"; else
			printf '  missing:%s\n' "$missing" | fold -sw 96 | sed '2,$s/^/    /'
		fi
	fi
}

behaviour_coverage "creatures/monsters/monster.cpp" "internal/game" "Monster"
behaviour_coverage "creatures/npcs/npc.cpp" "internal/game" "Npc"
behaviour_coverage "map/house/house.cpp" "internal/game" "House"
behaviour_coverage "items/decay/decay.cpp" "internal/game" "Decay" "DecayManager" "DecayManager"
behaviour_coverage "creatures/monsters/spawns/spawn_monster.cpp" "internal/game" "SpawnMonster" "SpawnEngine" "SpawnEngine"

echo
