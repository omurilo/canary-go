package game

import "github.com/opentibiabr/canary-go/internal/netmsg"

// Session is implemented by the game protocol connection so the world can push
// updates to a player's client.
type Session interface {
	SendToClient(w *netmsg.Writer)
	Player() *Player
}

// Skill indexes match the client skill order.
type Skill int

const (
	SkillFist Skill = iota
	SkillClub
	SkillSword
	SkillAxe
	SkillDistance
	SkillShielding
	SkillFishing
	SkillCount
)

// Player is a logged-in character. It embeds creature-like fields directly to
// keep the model flat for now.
type Player struct {
	ID        uint32 // creature id (assigned at spawn)
	DBID      uint32 // players.id
	AccountID uint32
	Name      string

	Pos       Position
	Direction Direction

	Level      uint16
	Experience uint64
	Health     uint32
	MaxHealth  uint32
	Mana       uint32
	MaxMana    uint32
	Soul       uint8
	Capacity   uint32 // free capacity (in the client unit)
	Speed      uint16
	Vocation   uint16
	Sex        uint8

	MagLevel uint16
	Skills   [SkillCount]uint16

	Outfit Outfit

	LightLevel uint8
	LightColor uint8

	// Inventory holds equipment slots 1..10 (CONST_SLOT_HEAD..CONST_SLOT_AMMO);
	// index 0 is unused. Persistence of these is a later milestone.
	Inventory [11]*Item

	Session Session
}

// GamemasterOutfit sets a default outfit if none was loaded.
func (p *Player) ensureDefaults() {
	if p.Outfit.LookType == 0 {
		p.Outfit.LookType = 128 // default male citizen
	}
	if p.MaxHealth == 0 {
		p.MaxHealth, p.Health = 150, 150
	}
	if p.Speed == 0 {
		p.Speed = 220
	}
	if p.Level == 0 {
		p.Level = 1
	}
	if p.Capacity == 0 {
		p.Capacity = 400
	}
	for i := range p.Skills {
		if p.Skills[i] == 0 {
			p.Skills[i] = 10
		}
	}
}
