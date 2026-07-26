package luaengine
import (
	"testing"
	"github.com/opentibiabr/canary-go/internal/game"
	"log/slog"
)
func TestItemTypeCall(t *testing.T) {
	e := New(&game.World{}, slog.Default())
	err := e.L.DoString(`
		local it = ItemType(2160)
		print("ItemType 2160:", type(it))
	`)
	if err != nil {
		t.Fatal(err)
	}
}
