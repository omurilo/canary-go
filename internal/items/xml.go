package items

import (
	"encoding/xml"
	"fmt"
	"os"
)

type xmlItems struct {
	Items []xmlItem `xml:"item"`
}

type xmlItem struct {
	ID         uint16         `xml:"id,attr"`
	FromID     uint16         `xml:"fromid,attr"`
	ToID       uint16         `xml:"toid,attr"`
	Attributes []xmlAttribute `xml:"attribute"`
}

type xmlAttribute struct {
	Key   string `xml:"key,attr"`
	Value string `xml:"value,attr"`
}

// LoadXML merges items.xml attributes (like SlotPosition, SlotType, WeaponType) into the catalog.
func (c *Catalog) LoadXML(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("items xml: read %s: %w", path, err)
	}

	var root xmlItems
	if err := xml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("items xml: unmarshal: %w", err)
	}

	for _, item := range root.Items {
		var ids []uint16
		if item.ID != 0 {
			ids = append(ids, item.ID)
		} else if item.FromID != 0 && item.ToID != 0 {
			for id := item.FromID; id <= item.ToID; id++ {
				ids = append(ids, id)
			}
		}

		for _, id := range ids {
			it := c.Get(id)
			if it == nil {
				continue
			}
			for _, attr := range item.Attributes {
				switch attr.Key {
				case "slotType":
					it.SlotType = attr.Value
				case "weaponType":
					it.WeaponType = attr.Value
				case "slot":
					it.SlotPosition = attr.Value
				case "floorchange":
					it.FloorChange = attr.Value
				case "forceuse":
					it.ForceUse = (attr.Value == "1")
				}
			}
		}
	}

	return nil
}
