#!/usr/bin/env bash
# Measures how close a ported method is to the C++ one it claims to be, beyond
# the fact that a function with that name exists.
#
# scripts/parity.sh answers "does a counterpart exist". That is a syntactic
# question and it can be satisfied by a one-line stub. This asks two harder ones:
#
#   1. Is the Go method REACHABLE? A method nothing calls is not parity, it is
#      dead code that happens to have the right name. This port has shipped
#      plenty: the whole npc_shop.go guard set existed with no caller.
#
#   2. Does it make roughly as many DECISIONS? Decision points (if / else if /
#      switch case / && / || / ternary / loops) are a crude but hard-to-fake
#      proxy for how much of the original logic survived. A 40-branch C++
#      function reimplemented in 3 branches did not survive.
#
# Both are proxies and both can be wrong in either direction: Go needs fewer
# branches than C++ for the same logic (no null checks on references, no manual
# iterator handling), and a faithful port of a simple function scores low on
# nothing. Read a low ratio as "go and look", not as a verdict. The number that
# means something is the one that is near zero.
#
# Usage: scripts/semantic-parity.sh [--md]
set -o pipefail

GO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="$(cd "$GO_ROOT/.." && pwd)/src"
[[ -d "$SRC" ]] || {
	echo "C++ source not found at $SRC" >&2
	exit 1
}

MD=0
[[ "${1:-}" == "--md" ]] && MD=1

# decisions counts branch points in a body on stdin. Comments are stripped first
# so a long explanation does not inflate the count of either side.
decisions() {
	sed -e 's://.*::' -e 's:/\*.*\*/::' |
		grep -oE '\bif\b|\bfor\b|\bwhile\b|\bcase\b|\bswitch\b|&&|\|\||\?' |
		wc -l | tr -d ' '
}

# cpp_body prints the body of Class::name, from its signature line to the first
# body prints a function body: from the line matching the (grep-syntax) pattern
# to the first closing brace in column 0.
#
# The start line is found with grep and handed to awk as a NUMBER. Passing the
# pattern into awk instead means the regex survives two levels of escaping —
# shell then awk string literals — and `\*` silently degrades to `*`, which awk
# then rejects as a bare quantifier.
body() { # file pattern
	local start
	start=$(grep -nE "$2" "$1" 2>/dev/null | head -1 | cut -d: -f1)
	[[ -z "$start" ]] && return
	tail -n "+$start" "$1" | awk 'NR>1 && /^}/ { print; exit } { print }'
}

cpp_body() { # file class name
	body "$1" "^[A-Za-z_].*\b$2::$3\("
}

go_body() { # dir recv name
	local f
	f=$(grep -rlE "^func (\([a-z]+ \*$2\) )?$3\(" "$1" --include='*.go' 2>/dev/null |
		grep -v '_test.go' | head -1)
	[[ -z "$f" ]] && return
	body "$f" "^func (\([a-z]+ \*$2\) )?$3\("
}

# callers counts non-test, non-defining references to a Go method name, across
# the WHOLE module.
#
# Scoping this to the defining package reported half the ported methods as dead
# when they are in fact called from internal/luaengine or internal/protocol —
# which is where most of this behaviour is reached from.
callers() { # _ name
	# Two shapes count:
	#
	#   `\.Name\b`  — a method call or a method VALUE. Not `\.Name\(`: a method
	#                 handed to the dispatcher (d.CheckDecay) is a caller, and
	#                 requiring the paren reported it dead.
	#   `\bName\(`  — a plain function. Not everything upstream models as a
	#                 method is one here: getPushItemLocationOptions is a free
	#                 function in Go and the dot-only pattern could not see it.
	#
	# The `func ` filter drops the definition line. It also drops a call that
	# shares a line with a function definition, which is why one-line method
	# bodies get split rather than left compact.
	# Comments are stripped and the match re-checked. Widening the pattern to bare
	# `Name(` made prose count: a comment reading "getType():isRewardBoss()" was
	# reporting a method as reachable that nothing calls.
	grep -rE "(\.$2\b|\b$2\()" "$GO_ROOT/internal" "$GO_ROOT/cmd" --include='*.go' 2>/dev/null |
		grep -v '_test.go' | grep -v "func " |
		sed -e 's://.*::' |
		grep -cE "(\.$2\b|\b$2\()" | tr -d ' '
}

# DEAD_OK lists methods that have no caller in the C++ source EITHER. Porting one
# and leaving it unreachable is 1:1, not a gap, and counting it as a gap means the
# number can never reach zero however much work is done.
#
# Every entry is verifiable: grep the name in ../src and find only its declaration
# and its definition. Re-check when upstream moves.
#
# Format: Class::method  reason
DEAD_OK="
SpawnMonster::removeMonster        no-caller-upstream:cleanup()-frees-slots-instead
SpawnMonster::isInSpawnMonsterZone no-caller-upstream:declared-and-never-used
SpawnMonster::getCenterPos         needs-zones:only-caller-is-zone.cpp:344
Monster::isRewardBoss              needs-reward-boss-system
Monster::getManaCost               needs-player-convince/summon
Monster::getLookCorpse             needs-corpse-description
"

# dead_expected reports whether Class::method is a known upstream-dead method.
dead_expected() { # class method
	printf '%s\n' "$DEAD_OK" | grep -q "^$1::$2 "
}

report() { # cpp-file class go-dir go-recv
	local cpp="$SRC/$1" class="$2" godir="$GO_ROOT/$3" recv="${4:-$2}"
	[[ -f "$cpp" ]] || return

	local name cppd god ratio thin=0 dead=0 expected=0 total=0 sum_c=0 sum_g=0
	local thin_list="" dead_list=""

	while read -r name; do
		# Go exports most of these, so the capitalised spelling has to be tried
		# too. Looking only for the C++ spelling found six methods out of a
		# hundred and reported the rest as "not comparable".
		local upper="$(printf '%s' "${name:0:1}" | tr '[:lower:]' '[:upper:]')${name:1}"
		local gbody
		gbody="$(go_body "$godir" "$recv" "$name")$(go_body "$godir" "$recv" "$upper")"
		[[ -z "$gbody" ]] && continue

		cppd=$(cpp_body "$cpp" "$class" "$name" | decisions)
		god=$(printf '%s' "$gbody" | decisions)
		total=$((total + 1))
		sum_c=$((sum_c + cppd))
		sum_g=$((sum_g + god))

		# Only interesting when the C++ side actually branches.
		if ((cppd >= 6)); then
			ratio=$((god * 100 / cppd))
			if ((ratio < 40)); then
				thin=$((thin + 1))
				thin_list="$thin_list $name($god/$cppd)"
			fi
		fi

		if [[ $(callers "$godir" "$upper") == 0 && $(callers "$godir" "$name") == 0 ]]; then
			if dead_expected "$class" "$name"; then
				expected=$((expected + 1))
			else
				dead=$((dead + 1))
				dead_list="$dead_list $name"
			fi
		fi
	done < <(grep -oE "(^|[^A-Za-z_])$class::[a-zA-Z_]+\(" "$cpp" | sed "s/.*$class:://;s/(//" | sort -u)

	((total == 0)) && return
	local dratio=0
	((sum_c > 0)) && dratio=$((sum_g * 100 / sum_c))

	local expected_note=""
	((expected > 0)) && expected_note=" (+$expected dead upstream too)"

	if ((MD)); then
		printf '| %s | %s | %s | %s | %s | %s |\n' "$class" "$total" "$sum_c" "$sum_g" "$dratio%" "$thin / $dead$expected_note"
	else
		printf '  %-14s compared %-4s decisions C++ %-5s Go %-5s %s%%   thin %-3s dead %s%s\n' \
			"$class" "$total" "$sum_c" "$sum_g" "$dratio" "$thin" "$dead" "$expected_note"
	fi
	[[ -n "$thin_list" ]] && printf '     thin:%s\n' "$thin_list" | fold -sw 100 | sed '2,$s/^/       /'
	[[ -n "$dead_list" ]] && printf '     never called:%s\n' "$dead_list" | fold -sw 100 | sed '2,$s/^/       /'
	return 0
}

if ((MD)); then
	printf '\n### Semantic parity\n\n| class | methods | C++ decisions | Go decisions | ratio | thin/dead |\n|---|---|---|---|---|---|\n'
else
	printf '\n== Semantic parity (decision density, and whether the port is reachable) ==\n'
fi

report "creatures/monsters/monster.cpp" "Monster" "internal/game" "Monster"
report "creatures/npcs/npc.cpp" "Npc" "internal/game" "Npc"
report "map/house/house.cpp" "House" "internal/game" "House"
report "items/decay/decay.cpp" "Decay" "internal/game" "DecayManager"
report "creatures/monsters/spawns/spawn_monster.cpp" "SpawnMonster" "internal/game" "SpawnBlock"

echo
