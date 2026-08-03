package game

import "testing"

// ResolveRespawnTown mirrors canary's IOLoginData::loadPlayer town fallback:
// own town, then Thais, then the first valid town, then the default spawn.
func TestResolveRespawnTownCanaryOrder(t *testing.T) {
	w := NewWorld()
	// Map tiles for the towns.
	tile := func(x, y uint16) { w.Map.SetTile(Position{X: x, Y: y, Z: 7}, &Tile{Ground: &Item{ID: 1}}) }
	tile(100, 100) // Thais
	tile(200, 200) // Edron
	tile(300, 300) // Ab'Dendriel

	w.Towns = map[string]Position{
		"thais":        {X: 100, Y: 100, Z: 7},
		"edron":        {X: 200, Y: 200, Z: 7},
		"ab'dendriel":  {X: 300, Y: 300, Z: 7},
	}
	w.TownsByID = map[uint16]Position{
		8:  {X: 100, Y: 100, Z: 7}, // Thais
		11: {X: 200, Y: 200, Z: 7}, // Edron
		5:  {X: 300, Y: 300, Z: 7}, // Ab'Dendriel
	}
	w.DefaultSpawn = Position{X: 500, Y: 500, Z: 7}

	// Valid town id -> that town.
	if got := w.ResolveRespawnTown(11, nil); got != (Position{X: 200, Y: 200, Z: 7}) {
		t.Errorf("valid town 11 = %v, want Edron", got)
	}
	// Unknown town id -> Thais (canary's fallback), not the first valid town.
	if got := w.ResolveRespawnTown(99, nil); got != (Position{X: 100, Y: 100, Z: 7}) {
		t.Errorf("unknown town = %v, want Thais", got)
	}
}
