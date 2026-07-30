package luaengine

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

// The three merchants that declared buy prices without an onBuyItem callback and so
// could not be bought from once delivery moved to the callback.
func TestPreviouslyUnbuyableMerchantsNowDispatch(t *testing.T) {
	datapack := filepath.Join("..", "..", "data-otservbr-global")
	core := filepath.Join("..", "..", "data")
	if _, err := os.Stat(filepath.Join(datapack, "npc", "elgar.lua")); err != nil {
		t.Skip("datapack not available")
	}

	for _, tc := range []struct{ file, name string }{
		{"elgar.lua", "elgar"},
		{"murim.lua", "murim"},
		{"enpa-deia_pema.lua", "enpa-deia pema"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := game.NewWorld()
			w.TypeRegistry = creatures.NewTypeRegistry()
			e := New(w, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
			defer e.Close()
			e.L.SetGlobal("DATA_DIRECTORY", lua.LString(datapack))
			e.L.SetGlobal("CORE_DIRECTORY", lua.LString(core))
			walkLoad(t, e, filepath.Join(datapack, "lib"))
			for _, sub := range []string{"lib", "libs", "npclib"} {
				walkLoad(t, e, filepath.Join(core, sub))
			}
			if err := e.DoFile(filepath.Join(datapack, "npc", tc.file)); err != nil {
				t.Fatalf("load %s: %v", tc.file, err)
			}

			nt := w.TypeRegistry.Npcs[tc.name]
			if nt == nil {
				t.Fatalf("npc type %q not registered", tc.name)
			}
			if len(nt.ShopItems) == 0 {
				t.Fatal("expected a shop")
			}
			// The registry must now carry the callback.
			npc := game.NewNpc(1, nt.Name, nt)
			if e.npcCallback(npc, "onBuyItem") == nil {
				t.Error("onBuyItem is still missing, so the merchant cannot be bought from")
			}
		})
	}
}
