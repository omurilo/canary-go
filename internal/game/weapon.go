package game

// Weapon defines an attack weapon for Lua scripts.
type Weapon struct {
	ID                 uint16
	Action             uint8
	Level              uint16
	MagicLevel         uint16
	Mana               uint32
	ManaPercent        uint32
	Health             int32
	HealthPercent      uint32
	Soul               uint8
	BreakChance        uint8
	Premium            bool
	WieldUnproperly    bool
	Vocations          []string
	ElementType        uint8
	ElementDamage      uint16
	Attack             int32
	Defense            int32
	ExtraDefense       int32
	Range              uint8
	Charges            uint8
	ShowCharges        bool
	Duration           uint32
	ShowDuration       bool
	DecayTo            uint16
	TransformEquipTo   uint16
	TransformDeEquipTo uint16
	SlotType           string
	SlotPosition       string
	HitChance          int8
	MaxHitChance       int8
	AmmoType           string
	ExtraElement       uint8
	ExtraElementDamage uint16
	// Wand/rod damage
	WandMinDamage int32
	WandMaxDamage int32
}
