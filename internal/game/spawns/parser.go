package spawns

import (
	"encoding/xml"
	"fmt"
	"os"
)

type SpawnsData struct {
	// Old format (spawns/spawn)
	Spawns []SpawnNode `xml:"spawn"`
	// New Canary format (monsters/monster)
	Monsters []SpawnNode `xml:"monster"`
	// New Canary format (npcs/npc)
	NPCs []SpawnNode `xml:"npc"`
}

type SpawnNode struct {
	CenterX  int            `xml:"centerx,attr"`
	CenterY  int            `xml:"centery,attr"`
	CenterZ  int            `xml:"centerz,attr"`
	Radius   int            `xml:"radius,attr"`
	Monsters []CreatureNode `xml:"monster"`
	NPCs     []CreatureNode `xml:"npc"`
}

type CreatureNode struct {
	Name      string `xml:"name,attr"`
	X         int    `xml:"x,attr"`
	Y         int    `xml:"y,attr"`
	Z         int    `xml:"z,attr"`
	SpawnTime int    `xml:"spawntime,attr"`
	Direction int    `xml:"dir,attr"`
}

func LoadSpawnFile(path string) (*SpawnsData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var spawns SpawnsData
	if err := xml.Unmarshal(data, &spawns); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return &spawns, nil
}
