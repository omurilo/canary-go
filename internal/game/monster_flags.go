package game

// The Monster:: predicates that read a type flag, ported from
// src/creatures/monsters/monster.cpp.
//
// Each one is two or three lines, which is why they were never written: they
// look like accessors. They are not. Every one of them has a condition on top
// of the flag — a summon is pushable whatever its type says, a challenged
// monster is hostile whatever its type says — and the callers that need them
// were reading the raw flag and getting the wrong answer for the summon and
// challenge cases.

// IsHostile is Monster::isHostile (monster.cpp:294).
func (m *Monster) IsHostile() bool {
	return m.Type != nil && m.Type.Flags.Hostile
}

// IsPushable is Monster::isPushable (monster.cpp:274): a summon can always be
// pushed, regardless of what its type declares. Without the summon clause a
// player's own summon blocks a corridor it should be shoved out of.
func (m *Monster) IsPushable() bool {
	if m.Master != nil {
		return true
	}
	return m.Type != nil && m.Type.Flags.Pushable
}

// IsAttackable is Monster::isAttackable (monster.cpp:278). A summon of a player
// is never attackable by that player's own side; the flag alone would allow it.
func (m *Monster) IsAttackable() bool {
	if _, ok := m.Master.(*Player); ok {
		return false
	}
	return m.Type == nil || m.Type.Flags.Attackable
}

// CanPushItems is Monster::canPushItems (monster.cpp:282): a familiar inherits
// the ability from its master rather than from its own type.
func (m *Monster) CanPushItems() bool {
	if master, ok := m.Master.(*Monster); ok && m.IsFamiliar() {
		return master.CanPushItems()
	}
	return m.Type != nil && m.Type.Flags.CanPushItems
}

// CanPushCreatures is Monster::canPushCreatures (monster.cpp:286).
func (m *Monster) CanPushCreatures() bool {
	if master, ok := m.Master.(*Monster); ok && m.IsFamiliar() {
		return master.CanPushCreatures()
	}
	return m.Type != nil && m.Type.Flags.CanPushCreatures
}

// IsRewardBoss is Monster::isRewardBoss (monster.cpp:290).
func (m *Monster) IsRewardBoss() bool {
	return m.Type != nil && m.Type.Flags.RewardBoss
}

// IsFamiliar is Monster::isFamiliar (monster.cpp:298): a summon owned by a
// player, as opposed to one owned by another monster.
func (m *Monster) IsFamiliar() bool {
	_, ok := m.Master.(*Player)
	return ok
}

// CanSeeInvisibility is Monster::canSeeInvisibility (monster.cpp:302). Upstream
// ties it to the monster being illusionable — the same property that lets it be
// disguised lets it see through a disguise.
func (m *Monster) CanSeeInvisibility() bool {
	return m.Type != nil && m.Type.Flags.Illusionable
}

// IsIgnoringFieldDamage is Monster::isIgnoringFieldDamage (monster.cpp:3131).
func (m *Monster) IsIgnoringFieldDamage() bool { return m.ignoreFieldDamage }

// IsInSpawnLocation is Monster::isInSpawnLocation (monster.cpp:1561). A monster
// with no spawn counts as being at home, which is what stops a summoned or
// scripted monster from trying to walk back to position zero.
func (m *Monster) IsInSpawnLocation() bool {
	return m.SpawnPosition == (Position{}) || m.GetPosition() == m.SpawnPosition
}

// GetFaction is Monster::getFaction (monster.cpp:259): a summon fights for its
// master's faction, not its own.
func (m *Monster) GetFaction() uint8 {
	if m.Master != nil {
		return CreatureFaction(m.Master)
	}
	if m.Type != nil {
		return m.Type.Faction
	}
	return FactionDefault
}

// GetManaCost is Monster::getManaCost (monster.cpp:322): the mana a player
// spends to summon or convince this monster.
func (m *Monster) GetManaCost() uint32 {
	if m.Type == nil {
		return 0
	}
	return m.Type.ManaCost
}

// GetLookCorpse is Monster::getLookCorpse (monster.cpp:877).
func (m *Monster) GetLookCorpse() uint16 {
	if m.Type == nil {
		return m.CorpseID
	}
	return m.Type.Corpse
}

// CanDropLoot is Monster::canDropLoot (monster.cpp:3944): the type's lootDrop
// flag, or any loot table at all. Upstream treats an empty table as "no loot"
// rather than as an error.
func (m *Monster) CanDropLoot() bool {
	if m.Type == nil {
		return false
	}
	return m.Type.Flags.LootDrop || len(m.Type.Loot) > 0
}

// GetLostExperience is Monster::getLostExperience (monster.cpp:872): the
// experience handed to the killers, scaled by the forge stack the same way its
// health was.
func (m *Monster) GetLostExperience() uint64 {
	if m.Type == nil {
		return 0
	}
	exp := float64(m.Type.Experience)
	if m.ForgeStack > 0 {
		exp *= 1 + float64(15*m.ForgeStack+35)/100
	}
	return uint64(exp)
}

// GetHealingCombatValue is Monster::getHealingCombatValue (monster.cpp:334):
// how much a healing spell of this type restores. A monster listing a healing
// element resists or amplifies healing the same way it does damage.
func (m *Monster) GetHealingCombatValue(healingType uint32) int32 {
	if m.Type == nil || m.Type.Elements == nil {
		return 0
	}
	return int32(m.Type.Elements[healingType])
}

// SetNormalCreatureLight is Monster::setNormalCreatureLight (monster.cpp:3437):
// a monster emits the light its type declares, not the player default.
func (m *Monster) SetNormalCreatureLight() {
	if m.Type == nil {
		return
	}
	m.LightLevel = m.Type.LightLevel
	m.LightColor = m.Type.LightColor
}
