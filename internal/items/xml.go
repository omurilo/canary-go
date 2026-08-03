package items

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type xmlItems struct {
	Items []xmlItem `xml:"item"`
}

type xmlItem struct {
	ID         uint16         `xml:"id,attr"`
	FromID     uint16         `xml:"fromid,attr"`
	ToID       uint16         `xml:"toid,attr"`
	Name       string         `xml:"name,attr"`
	Article    string         `xml:"article,attr"`
	Attributes []xmlAttribute `xml:"attribute"`
}

type xmlAttribute struct {
	Key        string         `xml:"key,attr"`
	Value      string         `xml:"value,attr"`
	Attributes []xmlAttribute `xml:"attribute"`
}

func processAttr(it *ItemType, attr xmlAttribute) {
	switch attr.Key {
	case "name":
		it.Name = attr.Value
	case "article":
		it.Article = attr.Value
	case "description":
		it.Description = attr.Value
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
	case "type":
		it.TypeName = attr.Value
		// C++ resolves the same attribute through ItemTypesMap into it.type,
		// which is what ItemType:getType reports (item_parse.hpp:168).
		it.Type = ItemTypeByName(attr.Value)
		if attr.Value == "ladder" {
			it.IsLadder = true
		} else if attr.Value == "door" {
			it.IsDoor = true
		}
	case "weight":
		if v, err := strconv.ParseUint(attr.Value, 10, 32); err == nil {
			it.Weight = uint32(v)
		}
	case "armor":
		if v, err := strconv.ParseInt(attr.Value, 10, 32); err == nil {
			it.Armor = int32(v)
		}
	case "attack":
		if v, err := strconv.ParseInt(attr.Value, 10, 32); err == nil {
			it.Attack = int32(v)
		}
	case "defense":
		if v, err := strconv.ParseInt(attr.Value, 10, 32); err == nil {
			it.Defense = int32(v)
		}
	case "extradefense":
		if v, err := strconv.ParseInt(attr.Value, 10, 32); err == nil {
			it.ExtraDefense = int32(v)
		}
	case "decayto", "decayTo":
		if v, err := strconv.ParseUint(attr.Value, 10, 16); err == nil {
			it.DecayTo = uint16(v)
		}
	case "duration":
		if v, err := strconv.ParseUint(attr.Value, 10, 32); err == nil {
			it.Duration = uint32(v)
		}
	case "showduration", "showDuration":
		it.ShowDuration = (attr.Value == "1")
	case "charges":
		if v, err := strconv.ParseUint(attr.Value, 10, 32); err == nil {
			it.Charges = uint32(v)
		}
	case "showcharges", "showCharges":
		it.ShowCharges = (attr.Value == "1")
	case "capacity", "containersize", "containerSize":
		// Container slot count. Canary's items.xml uses `containersize`; the
		// older `capacity` key is accepted too. Drives the 0x6E open-container
		// window's free-slot count.
		if v, err := strconv.ParseUint(attr.Value, 10, 32); err == nil {
			it.Capacity = uint32(v)
		}
	case "maxhitchance", "maxHitChance":
		if v, err := strconv.ParseInt(attr.Value, 10, 32); err == nil {
			it.MaxHitChance = int32(v)
		}
	case "hitchance", "hitChance":
		if v, err := strconv.ParseInt(attr.Value, 10, 32); err == nil {
			it.HitChance = int32(v)
		}
	case "range":
		if v, err := strconv.ParseInt(attr.Value, 10, 32); err == nil {
			it.Range = int32(v)
		}
	case "shoottype", "shootType":
		// An unknown name leaves the field at CONST_ANI_NONE, like the C++ parser,
		// rather than defaulting to an arrow.
		if st, ok := ShootTypeByName(attr.Value); ok {
			it.ShootType = st
		}
	case "ammotype", "ammoType":
		it.AmmoType = attr.Value
	case "primarytype", "primaryType":
		if attr.Value == "quivers" {
			it.IsQuiver = true
		} else if attr.Value == "ammunition" {
			it.WeaponType = "ammunition"
		}
	case "transformequipto", "transformEquipTo":
		if v, err := strconv.ParseUint(attr.Value, 10, 16); err == nil {
			it.TransformEquipTo = uint16(v)
		}
	case "transformdeequipto", "transformDeEquipTo":
		if v, err := strconv.ParseUint(attr.Value, 10, 16); err == nil {
			it.TransformDeEquipTo = uint16(v)
		}
	case "maletransformto":
		if v, err := strconv.ParseUint(attr.Value, 10, 16); err == nil {
			it.TransformToOnUseMale = uint16(v)
		}
	case "femaletransformto":
		if v, err := strconv.ParseUint(attr.Value, 10, 16); err == nil {
			it.TransformToOnUseFemale = uint16(v)
		}
	case "fluidsource", "fluidSource":
		var fluidType uint16
		switch strings.ToLower(attr.Value) {
		case "water":
			fluidType = 1
		case "wine":
			fluidType = 2
		case "beer":
			fluidType = 3
		case "mud":
			fluidType = 4
		case "blood":
			fluidType = 5
		case "slime":
			fluidType = 6
		case "oil":
			fluidType = 7
		case "urine":
			fluidType = 8
		case "milk":
			fluidType = 9
		case "mana":
			fluidType = 10
		case "life":
			fluidType = 11
		case "lemonade":
			fluidType = 12
		case "rum":
			fluidType = 13
		case "fruitjuice", "juice":
			fluidType = 14
		case "coconutmilk":
			fluidType = 15
		case "mead":
			fluidType = 16
		case "tea":
			fluidType = 17
		case "ink":
			fluidType = 18
		}
		it.FluidSource = fluidType
	case "augments":
		it.Augments = parseAugments(attr.Attributes)
	default:
		// Attempt to parse as int for Stats (e.g., skillSword, absorbpercentfire, elementice)
		if v, err := strconv.ParseInt(attr.Value, 10, 32); err == nil {
			if it.Stats == nil {
				it.Stats = make(map[string]int32)
			}
			it.Stats[strings.ToLower(attr.Key)] = int32(v)
		}
	}
	for _, child := range attr.Attributes {
		processAttr(it, child)
	}
}

// parseAugments extracts structured augment data from nested XML attributes.
// Expected XML format:
//
//	<attribute key="augments">
//	  <attribute key="spellname" value="Spell Name" />
//	  <attribute key="type" value="1" />
//	  <attribute key="value" value="25" />
//	</attribute>
func parseAugments(attrs []xmlAttribute) []AugmentInfo {
	var augments []AugmentInfo
	for _, attr := range attrs {
		var aug AugmentInfo
		aug.SpellName = extractChildValue(attr.Attributes, "spellname")
		if aug.SpellName == "" {
			continue
		}
		typeVal := extractChildValue(attr.Attributes, "type")
		if v, err := strconv.ParseUint(typeVal, 10, 8); err == nil {
			aug.Type = AugmentType(v)
		}
		valueVal := extractChildValue(attr.Attributes, "value")
		if v, err := strconv.ParseInt(valueVal, 10, 32); err == nil {
			aug.Value = int32(v)
		}
		augments = append(augments, aug)
	}
	return augments
}

// extractChildValue searches a slice of xmlAttribute for one with the given key
// and returns its Value.
func extractChildValue(attrs []xmlAttribute, key string) string {
	for _, a := range attrs {
		if a.Key == key {
			return a.Value
		}
	}
	return ""
}

// LoadXML merges items.xml attributes (like SlotPosition, SlotType, WeaponType) into the catalog.
func (c *Catalog) LoadXML(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("items xml: read %s: %w", path, err)
	}

	var root xmlItems
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		return input, nil
	}
	if err := decoder.Decode(&root); err != nil {
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
			if item.Name != "" {
				it.Name = item.Name
			}
			if item.Article != "" {
				it.Article = item.Article
			}
			for _, attr := range item.Attributes {
				processAttr(it, attr)
			}
		}
	}

	return nil
}
