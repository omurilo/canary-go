package game

import "github.com/opentibiabr/canary-go/internal/creatures"

type Npc struct {
	BaseCreature
	// interactions tracks players currently in a conversation with this NPC
	// (playerID → topic). NpcHandler:checkInteraction gates trade/keyword
	// navigation on this, so it must be real (not a stub) for post-greeting
	// interaction to work. Accessed only under the Lua engine lock.
	interactions map[uint32]int

	// Type is the definition this NPC was spawned from; the think loop reads its
	// walk and voice settings.
	Type *creatures.NpcType

	// MasterPos is the spawn position. Walking stays within WalkRadius of it,
	// matching Npc::isInSpawnRange against masterPos.
	MasterPos Position

	// Tick accumulators for onThinkWalk and onThinkYell.
	walkTicks uint32
	yellTicks uint32
}

// SetPlayerInteraction marks playerID as interacting with the NPC at the given
// dialogue topic.
func (n *Npc) SetPlayerInteraction(playerID uint32, topic int) {
	if n.interactions == nil {
		n.interactions = make(map[uint32]int)
	}
	n.interactions[playerID] = topic
}

// RemovePlayerInteraction ends playerID's conversation with the NPC.
func (n *Npc) RemovePlayerInteraction(playerID uint32) {
	delete(n.interactions, playerID)
}

// IsInteractingWithPlayer reports whether playerID is mid-conversation.
func (n *Npc) IsInteractingWithPlayer(playerID uint32) bool {
	_, ok := n.interactions[playerID]
	return ok
}

// InteractingPlayers returns a slice of player IDs currently interacting with this NPC.
func (n *Npc) InteractingPlayers() []uint32 {
	if len(n.interactions) == 0 {
		return nil
	}
	ids := make([]uint32, 0, len(n.interactions))
	for id := range n.interactions {
		ids = append(ids, id)
	}
	return ids
}

func NewNpc(id uint32, name string, nType *creatures.NpcType) *Npc {
	maxHealth := uint32(100)
	speed := uint32(100)
	outfit := Outfit{}

	if nType != nil {
		maxHealth = nType.MaxHealth
		speed = nType.Speed
		// NpcInfo defaults baseSpeed to 55 (src/creatures/npcs/npcs.hpp:41). No npc
		// script in the datapack sets speed, so without this fallback every NPC has
		// speed 0 — and onThinkWalk bails out on baseSpeed == 0, so none would ever
		// take a step.
		if speed == 0 {
			speed = 55
		}
		outfit = Outfit{
			LookType:   nType.Outfit.LookType,
			LookTypeEx: nType.Outfit.LookTypeEx,
			Head:       nType.Outfit.Head,
			Body:       nType.Outfit.Body,
			Legs:       nType.Outfit.Legs,
			Feet:       nType.Outfit.Feet,
			Addons:     nType.Outfit.Addons,
			LookMount:  nType.Outfit.LookMount,
		}
	}

	return &Npc{
		BaseCreature: BaseCreature{
			ID:        id,
			Name:      name,
			Health:    maxHealth,
			MaxHealth: maxHealth,
			Speed:     uint16(speed),
			Outfit:    outfit,
		},
		Type: nType,
	}
}

// SpeechBubble returns the SPEECHBUBBLE_* icon the client should draw, defaulting
// to NORMAL when the type carries none.
func (n *Npc) SpeechBubble() uint8 {
	if n.Type == nil || n.Type.SpeechBubble == 0 {
		return creatures.SpeechBubbleNormal
	}
	return n.Type.SpeechBubble
}

// CurrencyID returns the item the NPC trades in.
func (n *Npc) CurrencyID() uint16 {
	if n.Type == nil || n.Type.CurrencyID == 0 {
		return creatures.DefaultNpcCurrency
	}
	return n.Type.CurrencyID
}

// TurnToCreature faces the NPC toward a creature, mirroring Npc::turnToCreature.
func (n *Npc) TurnToCreature(c Creature) {
	if c == nil {
		return
	}
	n.SetDirection(getDirectionTo(n.GetPosition(), c.GetPosition()))
}

func (n *Npc) GetCreatureType() uint8 { return 2 } // CREATURETYPE_NPC
