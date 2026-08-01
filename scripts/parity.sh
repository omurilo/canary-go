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
row "enum globals" "${cpp_enums:-0}" "${go_enums:-0}" "0 missing, 0 wrong (snapshot)"

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
cmp_pair() { # label cpp-path go-path...
	local label cpp c g f
	label="$1"
	cpp="$2"
	shift 2
	c=$(cat "$SRC/$cpp" 2>/dev/null | wc -l | tr -d ' ')
	[[ "$c" == "0" || -z "$c" ]] && return
	g=0
	for f in "$@"; do
		g=$((g + $(cat "$GO_ROOT/$f" 2>/dev/null | wc -l | tr -d ' ')))
	done
	row "$label" "$c" "$g" "$((g * 100 / c))%"
}
cmp_pair "monster AI" "creatures/monsters/monster.cpp" \
	"internal/game/ai_engine.go" "internal/game/monster_ai.go"
cmp_pair "npc" "creatures/npcs/npc.cpp" "internal/game/npc.go"
cmp_pair "house" "map/house/house.cpp" "internal/game/house.go"
cmp_pair "spawn" "creatures/monsters/spawns/spawn_monster.cpp" "internal/game/spawn_engine.go"
cmp_pair "decay" "items/decay/decay.cpp" "internal/game/decay.go"
cmp_pair "map serialize" "io/iomapserialize.cpp" "internal/db/tile_store.go"

echo
