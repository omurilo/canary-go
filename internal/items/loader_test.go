package items_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opentibiabr/canary-go/internal/items"
	"github.com/opentibiabr/canary-go/internal/protobuf/appearances"
	"google.golang.org/protobuf/proto"
)

func TestLoadFromProtobuf(t *testing.T) {
	// Create mock appearances
	id := uint32(100)
	name := []byte("gold coin")
	desc := []byte("It is a gold coin.")
	cumulative := true
	
	app := &appearances.Appearances{
		Object: []*appearances.Appearance{
			{
				Id: &id,
				Name: name,
				Description: desc,
				Flags: &appearances.AppearanceFlags{
					Cumulative: &cumulative,
				},
			},
		},
	}

	data, err := proto.Marshal(app)
	if err != nil {
		t.Fatalf("failed to marshal mock proto: %v", err)
	}

	tempDir := t.TempDir()
	datPath := filepath.Join(tempDir, "appearances.dat")
	if err := os.WriteFile(datPath, data, 0644); err != nil {
		t.Fatalf("failed to write dat file: %v", err)
	}

	catalog := items.NewCatalog()
	if err := catalog.LoadFromProtobuf(datPath); err != nil {
		t.Fatalf("LoadFromProtobuf failed: %v", err)
	}

	if len(catalog.Items) <= 100 {
		t.Fatalf("catalog size too small: %d", len(catalog.Items))
	}

	item := catalog.Items[100]
	if item.Name != "gold coin" {
		t.Errorf("expected name 'gold coin', got %q", item.Name)
	}
	if !item.Stackable {
		t.Errorf("expected item to be stackable")
	}
	if item.ID != 100 {
		t.Errorf("expected ID 100, got %d", item.ID)
	}
}
