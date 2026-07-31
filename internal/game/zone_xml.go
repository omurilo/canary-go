package game

import (
	"encoding/xml"
	"fmt"
	"os"
)

// zonesFile is the `<map>-zones.xml` document: it maps map-editor zone ids to
// names and nothing else. The positions come from the OTBM tiles.
//
//	<zones>
//	  <zone name="hazard-area" zoneid="1" />
//	</zones>
type zonesFile struct {
	XMLName xml.Name    `xml:"zones"`
	Zones   []zoneEntry `xml:"zone"`
}

type zoneEntry struct {
	Name   string `xml:"name,attr"`
	ZoneID uint32 `xml:"zoneid,attr"`
}

// LoadZonesFromXML names the zones declared in path, the port of
// Zone::loadFromXML (src/game/zones/zone.cpp). shiftID left-shifts every id, which
// is how a second map's zones are kept from colliding with the main map's.
//
// It is safe to call after the OTBM has already claimed ids: ZoneRegistry.Add
// renames the existing zone rather than creating a second one, exactly as the
// upstream linking branch does.
func (r *ZoneRegistry) LoadZonesFromXML(path string, shiftID uint16) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var doc zonesFile
	if err := xml.Unmarshal(raw, &doc); err != nil {
		return 0, fmt.Errorf("parse %s: %w", path, err)
	}
	named := 0
	var firstErr error
	for _, z := range doc.Zones {
		if z.Name == "" {
			continue
		}
		if _, err := r.Add(z.Name, z.ZoneID<<shiftID); err != nil {
			// A duplicate name or the reserved "default" is a datapack problem, not a
			// reason to abandon the rest of the file.
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		named++
	}
	return named, firstErr
}

// ApplyZonePositions attaches the positions the OTBM recorded per zone id. Ids the
// XML has not named yet get an unnamed zone, which LoadZonesFromXML can name later
// (ZoneRegistry.ByID auto-creates, like Zone::getZone(uint32_t)).
func (r *ZoneRegistry) ApplyZonePositions(byZoneID map[uint16][]Position) {
	for id, positions := range byZoneID {
		z := r.ByID(uint32(id))
		if z == nil {
			continue
		}
		for _, pos := range positions {
			z.AddPosition(pos)
		}
	}
}
