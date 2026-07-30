#!/usr/bin/env python3
"""Extract the enum globals lua_enums.cpp registers, resolve their values from the
C++ headers, and report which ones canary-go's enums.go is missing."""
import re, os, sys, json, glob

SRC = "/Users/murilo.alves/projects/personal/canary/src"
LUA_ENUMS = os.path.join(SRC, "lua/functions/core/game/lua_enums.cpp")

# ---- 1. what gets registered ------------------------------------------------
text = open(LUA_ENUMS).read()
# strip the macro definitions themselves (they contain the literal token)
text = text.split("void LuaEnums::init", 1)[1]

regs = []  # (lua_name, cpp_symbol)
for m in re.finditer(r'registerEnum\(\s*L\s*,\s*([A-Za-z_][\w:]*)\s*\)', text):
    sym = m.group(1)
    regs.append((sym.split("::")[-1], sym))
# luaNamespace is a local std::string reassigned as the file walks through the
# enum groups, so the prefix has to be tracked positionally.
# soundNamespace is a file-scope constexpr; luaNamespace is a reassigned local.
consts = dict(re.findall(r'constexpr const char\* (\w+)\s*=\s*"([^"]*)"', open(LUA_ENUMS).read()))
ns = None
for line in text.splitlines():
    a = re.search(r'luaNamespace\s*=\s*"([^"]*)"', line)
    if a:
        ns = a.group(1)
    b = re.search(r'registerEnumNamespace\(\s*L\s*,\s*(?:(\w+)|"([^"]*)")\s*,\s*([A-Za-z_][\w:]*)\s*\)', line)
    if b:
        var, lit, sym = b.group(1), b.group(2), b.group(3)
        pfx = lit if lit is not None else (consts.get(var) if var in consts else ns)
        if pfx is None:
            continue
        regs.append((pfx + sym.split("::")[-1], sym))

# ---- 2. build the symbol table from every enum body in src/ -----------------
# enumerators are stored twice: bare name and Enum::name
values = {}       # "Enum::NAME" -> int
bare = {}         # "NAME" -> int (first definition wins, like a flat namespace)
enum_re = re.compile(
    r'enum\s+(?:class\s+|struct\s+)?([A-Za-z_]\w*)?\s*(?::\s*[\w:\s]+)?\s*\{(.*?)\}\s*;',
    re.S)

def strip_comments(s):
    s = re.sub(r'/\*.*?\*/', '', s, flags=re.S)
    s = re.sub(r'//[^\n]*', '', s)
    return s

def evaluate(expr, local, enum_name):
    expr = expr.strip()
    if not expr:
        return None
    # C++ suffixes and casts we can ignore
    expr = re.sub(r'\b(\d+)[uUlL]+\b', r'\1', expr)
    expr = re.sub(r'static_cast<[^>]+>', '', expr)
    expr = expr.replace("'", "")
    # resolve identifiers
    def sub_ident(m):
        name = m.group(0)
        if re.fullmatch(r'0[xX][0-9a-fA-F]+|\d+', name):
            return name
        short = name.split("::")[-1]
        for key in (name, f"{enum_name}::{short}"):
            if key in values:
                return str(values[key])
        if short in local:
            return str(local[short])
        if short in bare:
            return str(bare[short])
        return "None"
    expr = re.sub(r'[A-Za-z_][\w:]*', sub_ident, expr)
    if "None" in expr:
        return None
    try:
        v = eval(expr, {"__builtins__": {}}, {})
        return int(v)
    except Exception:
        return None

headers = []
for ext in ("hpp", "h", "cpp"):
    headers += glob.glob(os.path.join(SRC, "**", f"*.{ext}"), recursive=True)

for path in headers:
    try:
        body = strip_comments(open(path, errors="ignore").read())
    except OSError:
        continue
    for m in enum_re.finditer(body):
        enum_name = m.group(1) or ""
        local, nxt = {}, 0
        for raw in m.group(2).split(","):
            raw = raw.strip()
            if not raw:
                continue
            if "=" in raw:
                name, expr = raw.split("=", 1)
                name = name.strip()
                if not re.fullmatch(r'[A-Za-z_]\w*', name):
                    continue
                v = evaluate(expr, local, enum_name)
                if v is None:
                    nxt = None
                    continue
            else:
                name = raw
                if not re.fullmatch(r'[A-Za-z_]\w*', name):
                    continue
                if nxt is None:
                    continue
                v = nxt
            local[name] = v
            nxt = v + 1
            if enum_name:
                values.setdefault(f"{enum_name}::{name}", v)
            bare.setdefault(name, v)

# ---- 3. resolve the registered symbols -------------------------------------
resolved, unresolved = {}, []
for lua_name, sym in regs:
    short = sym.split("::")[-1]
    val = None
    if "::" in sym:
        val = values.get(sym)
        if val is None:
            qual = sym.split("::")[-2]
            val = values.get(f"{qual}::{short}")
    if val is None:
        val = bare.get(short)
    if val is None:
        unresolved.append((lua_name, sym))
    else:
        resolved[lua_name] = val

# ---- 4. diff against Go ----------------------------------------------------
go = open("/Users/murilo.alves/projects/personal/canary/canary-go/internal/luaengine/enums.go").read()
have = set(re.findall(r'"([A-Za-z][\w]*)"\s*:', go))

missing = {k: v for k, v in resolved.items() if k not in have}

print(f"registered in C++      : {len(regs)}")
print(f"distinct lua names     : {len(set(n for n, _ in regs))}")
print(f"values resolved        : {len(resolved)}")
print(f"unresolved             : {len(unresolved)}")
print(f"already in Go          : {len(have)}")
print(f"missing from Go        : {len(missing)}")
json.dump({"missing": missing, "unresolved": unresolved, "resolved": resolved},
          open(sys.argv[1] if len(sys.argv) > 1 else "/tmp/enums.json", "w"), indent=1)
for n, s in unresolved[:15]:
    print("  unresolved:", n, "<-", s)
