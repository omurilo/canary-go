package game

// Item is a minimal item instance: a client item id plus an optional stack
// count/subtype. Full item attributes (the OTBR blob) are a later milestone.
type Item struct {
	ID         uint16
	Count      uint16
	Attributes []byte

	// Contents holds the items inside a container item (chest, bag, ...), in
	// stack order. Empty for non-containers.
	Contents []*Item
}

// Outfit describes a creature's appearance.
type Outfit struct {
	LookType   uint16
	Head       uint8
	Body       uint8
	Legs       uint8
	Feet       uint8
	Addons     uint8
	LookTypeEx uint16
	LookMount  uint16
	MountHead  uint8
	MountBody  uint8
	MountLegs  uint8
	MountFeet  uint8
}
