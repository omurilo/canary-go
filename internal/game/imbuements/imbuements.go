package imbuements

import (
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type BaseImbuement struct {
	ID              uint16
	Name            string
	Price           uint32
	ProtectionPrice uint32
	RemoveCost      uint32
	Duration        uint32
	Percent         uint8
}

type CategoryImbuement struct {
	ID        uint8
	Name      string
	Agressive bool
}

type ImbuementItem struct {
	ID    uint16
	Count uint16
}

type Imbuement struct {
	ID          uint16
	Name        string
	BaseID      uint16
	CategoryID  uint8
	IconID      uint16
	Premium     bool
	Description string
	SubGroup    string
	EffectType  string
	EffectValue int32
	Items       []ImbuementItem
	ScrollID    uint16
}

type Registry struct {
	bases      map[uint16]*BaseImbuement
	categories map[uint8]*CategoryImbuement
	imbuements map[uint16]*Imbuement
	byBase     map[uint16][]*Imbuement
}

func NewRegistry() *Registry {
	return &Registry{
		bases:      make(map[uint16]*BaseImbuement),
		categories: make(map[uint8]*CategoryImbuement),
		imbuements: make(map[uint16]*Imbuement),
		byBase:     make(map[uint16][]*Imbuement),
	}
}

func (r *Registry) GetBaseByID(id uint16) *BaseImbuement {
	return r.bases[id]
}

func (r *Registry) GetCategoryByID(id uint8) *CategoryImbuement {
	return r.categories[id]
}

func (r *Registry) GetImbuement(id uint16) *Imbuement {
	return r.imbuements[id]
}

func (r *Registry) GetImbuementsByBase(baseID uint16) []*Imbuement {
	return r.byBase[baseID]
}

func (r *Registry) GetAllImbuements() []*Imbuement {
	out := make([]*Imbuement, 0, len(r.imbuements))
	for _, imb := range r.imbuements {
		out = append(out, imb)
	}
	return out
}

type xmlImbuements struct {
	XMLName     xml.Name         `xml:"imbuements"`
	Bases       []xmlBase        `xml:"base"`
	Categories  []xmlCategory    `xml:"category"`
	ImbuementEl []xmlImbuementEl `xml:"imbuement"`
}

type xmlBase struct {
	ID              uint16 `xml:"id,attr"`
	Name            string `xml:"name,attr"`
	Price           uint32 `xml:"price,attr"`
	ProtectionPrice uint32 `xml:"protectionPrice,attr"`
	RemoveCost      uint32 `xml:"removecost,attr"`
	Duration        uint32 `xml:"duration,attr"`
	Percent         uint8  `xml:"percent,attr"`
}

type xmlCategory struct {
	ID        uint8  `xml:"id,attr"`
	Name      string `xml:"name,attr"`
	Agressive uint8  `xml:"agressive,attr"`
}

type xmlImbuementEl struct {
	Name       string       `xml:"name,attr"`
	Base       uint16       `xml:"base,attr"`
	Category   uint8        `xml:"category,attr"`
	SubGroup   string       `xml:"subgroup,attr"`
	IconID     uint16       `xml:"iconid,attr"`
	Premium    uint8        `xml:"premium,attr"`
	Storage    uint32       `xml:"storage,attr"`
	Attributes []xmlImbAttr `xml:"attribute"`
}

type xmlImbAttr struct {
	Key   string `xml:"key,attr"`
	Value string `xml:"value,attr"`
	Type  string `xml:"type,attr"`
	Count uint16 `xml:"count,attr"`
}

func LoadRegistry(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading imbuements xml: %w", err)
	}

	var parsed xmlImbuements
	if err := xml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("parsing imbuements xml: %w", err)
	}

	r := NewRegistry()

	for _, b := range parsed.Bases {
		r.bases[b.ID] = &BaseImbuement{
			ID:              b.ID,
			Name:            b.Name,
			Price:           b.Price,
			ProtectionPrice: b.ProtectionPrice,
			RemoveCost:      b.RemoveCost,
			Duration:        b.Duration,
			Percent:         b.Percent,
		}
	}

	for _, c := range parsed.Categories {
		r.categories[c.ID] = &CategoryImbuement{
			ID:        c.ID,
			Name:      c.Name,
			Agressive: c.Agressive != 0,
		}
	}

	nextID := uint16(1)
	for _, imb := range parsed.ImbuementEl {
		desc := ""
		var items []ImbuementItem
		scrollID := uint16(0)
		effectType := ""
		effectVal := int32(0)

		for _, attr := range imb.Attributes {
			switch attr.Key {
			case "description":
				desc = attr.Value
			case "effect":
				effectType = attr.Type
				if v, err := strconv.ParseInt(attr.Value, 10, 32); err == nil {
					effectVal = int32(v)
				}
			case "item":
				id, err := strconv.ParseUint(attr.Value, 10, 16)
				if err == nil {
					items = append(items, ImbuementItem{ID: uint16(id), Count: attr.Count})
				}
			case "scroll":
				if v, err := strconv.ParseUint(attr.Value, 10, 16); err == nil {
					scrollID = uint16(v)
				}
			}
		}

		id := nextID
		nextID++

		imbObj := &Imbuement{
			ID:          id,
			Name:        imb.Name,
			BaseID:      imb.Base,
			CategoryID:  imb.Category,
			SubGroup:    imb.SubGroup,
			IconID:      imb.IconID,
			Premium:     imb.Premium != 0,
			Description: desc,
			EffectType:  effectType,
			EffectValue: effectVal,
			Items:       items,
			ScrollID:    scrollID,
		}

		r.imbuements[id] = imbObj
		r.byBase[imb.Base] = append(r.byBase[imb.Base], imbObj)
	}

	return r, nil
}

func (r *Registry) GetImbuementByScrollID(scrollID uint16) *Imbuement {
	for _, imb := range r.imbuements {
		if imb.ScrollID == scrollID {
			return imb
		}
	}
	return nil
}

func (r *Registry) GetImbuementByNameAndBase(name string, baseID uint16) *Imbuement {
	for _, imb := range r.imbuements {
		if strings.EqualFold(imb.Name, name) && imb.BaseID == baseID {
			return imb
		}
	}
	return nil
}
