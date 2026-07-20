package creatures

import (
	"encoding/xml"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type Outfit struct {
	LookType  uint16
	Head      uint8
	Body      uint8
	Legs      uint8
	Feet      uint8
	Addons    uint8
	LookMount uint16
}

type MonsterType struct {
	Name      string
	Speed     uint32
	MaxHealth uint32
	Outfit    Outfit
}

type NpcType struct {
	Name      string
	Speed     uint32
	MaxHealth uint32
	Outfit    Outfit
}

type TypeRegistry struct {
	Monsters map[string]*MonsterType
	Npcs     map[string]*NpcType
}

func NewTypeRegistry() *TypeRegistry {
	return &TypeRegistry{
		Monsters: make(map[string]*MonsterType),
		Npcs:     make(map[string]*NpcType),
	}
}

type xmlMonster struct {
	Name   string    `xml:"name,attr"`
	Speed  uint32    `xml:"speed,attr"`
	Health xmlHealth `xml:"health"`
	Look   xmlLook   `xml:"look"`
}

type xmlNpc struct {
	Name   string    `xml:"name,attr"`
	Speed  uint32    `xml:"speed,attr"`
	Health xmlHealth `xml:"health"`
	Look   xmlLook   `xml:"look"`
}

type xmlHealth struct {
	Max uint32 `xml:"max,attr"`
	Now uint32 `xml:"now,attr"`
}

type xmlLook struct {
	Type   uint16 `xml:"type,attr"`
	Head   uint8  `xml:"head,attr"`
	Body   uint8  `xml:"body,attr"`
	Legs   uint8  `xml:"legs,attr"`
	Feet   uint8  `xml:"feet,attr"`
	Addons uint8  `xml:"addons,attr"`
	Mount  uint16 `xml:"mount,attr"`
}

func (r *TypeRegistry) LoadMonsters(dataDir string) error {
	return filepath.WalkDir(dataDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || filepath.Ext(path) != ".xml" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		var mon xmlMonster
		if err := xml.Unmarshal(data, &mon); err == nil && mon.Name != "" {
			mType := &MonsterType{
				Name:      mon.Name,
				Speed:     mon.Speed,
				MaxHealth: mon.Health.Max,
				Outfit: Outfit{
					LookType:  mon.Look.Type,
					Head:      mon.Look.Head,
					Body:      mon.Look.Body,
					Legs:      mon.Look.Legs,
					Feet:      mon.Look.Feet,
					Addons:    mon.Look.Addons,
					LookMount: mon.Look.Mount,
				},
			}
			if mType.MaxHealth == 0 {
				mType.MaxHealth = mon.Health.Now
			}
			if mType.MaxHealth == 0 {
				mType.MaxHealth = 100 // fallback
			}
			if mType.Speed == 0 {
				mType.Speed = 200 // default speed
			}
			r.Monsters[strings.ToLower(mon.Name)] = mType
		}

		return nil
	})
}

func (r *TypeRegistry) LoadNpcs(dataDir string) error {
	return filepath.WalkDir(dataDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || filepath.Ext(path) != ".xml" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		var npc xmlNpc
		if err := xml.Unmarshal(data, &npc); err == nil && npc.Name != "" {
			nType := &NpcType{
				Name:      npc.Name,
				Speed:     npc.Speed,
				MaxHealth: npc.Health.Max,
				Outfit: Outfit{
					LookType:  npc.Look.Type,
					Head:      npc.Look.Head,
					Body:      npc.Look.Body,
					Legs:      npc.Look.Legs,
					Feet:      npc.Look.Feet,
					Addons:    npc.Look.Addons,
					LookMount: npc.Look.Mount,
				},
			}
			if nType.MaxHealth == 0 {
				nType.MaxHealth = npc.Health.Now
			}
			if nType.MaxHealth == 0 {
				nType.MaxHealth = 100
			}
			if nType.Speed == 0 {
				nType.Speed = 100
			}
			r.Npcs[strings.ToLower(npc.Name)] = nType
		}

		return nil
	})
}
