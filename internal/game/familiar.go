package game

// Familiar represents a familiar creature that can be summoned by a player.
// Familiars are vocation-specific cosmetic companions with a look type.
// Mirrors C++ src/creatures/players/grouping/familiars.{hpp,cpp}.
type Familiar struct {
	LookType uint16
	Name     string
	Premium  bool
	Unlocked bool
	Type     string // "knight", "mage", "paladin", "druid"
}

// --- Player familiar methods ---

// AddFamiliar registers a familiar as unlocked for the player. Returns true
// if the familiar was newly unlocked. Mirrors Player::addFamiliar.
func (p *Player) AddFamiliar(lookType uint16) bool {
	if lookType == 0 {
		return false
	}
	for i := range p.Familiars {
		if p.Familiars[i].LookType == lookType {
			if !p.Familiars[i].Unlocked {
				p.Familiars[i].Unlocked = true
				return true
			}
			return false // already unlocked
		}
	}
	// Unknown look type — add with defaults
	p.Familiars = append(p.Familiars, Familiar{
		LookType: lookType,
		Unlocked: true,
	})
	return true
}

// RemoveFamiliar locks a familiar so it can no longer be summoned.
// Mirrors Player::removeFamiliar.
func (p *Player) RemoveFamiliar(lookType uint16) bool {
	for i := range p.Familiars {
		if p.Familiars[i].LookType == lookType {
			if p.Familiars[i].Unlocked {
				p.Familiars[i].Unlocked = false
				if p.ActiveFamiliar == lookType {
					p.ActiveFamiliar = 0
				}
				return true
			}
			return false
		}
	}
	return false
}

// HasFamiliar returns true if the player has the given familiar unlocked.
func (p *Player) HasFamiliar(lookType uint16) bool {
	for _, f := range p.Familiars {
		if f.LookType == lookType {
			return f.Unlocked
		}
	}
	return false
}

// GetFamiliarLooktype returns the currently active familiar's look type, or 0.
func (p *Player) GetFamiliarLooktype() uint16 {
	return p.ActiveFamiliar
}

// SetFamiliarLooktype sets the active familiar. The familiar must be unlocked.
// Pass 0 to dismiss the familiar.
func (p *Player) SetFamiliarLooktype(lookType uint16) bool {
	if lookType == 0 {
		p.ActiveFamiliar = 0
		return true
	}
	if !p.HasFamiliar(lookType) {
		return false
	}
	p.ActiveFamiliar = lookType
	return true
}

// UnlockedFamiliars returns the count of unlocked familiars.
func (p *Player) UnlockedFamiliars() int {
	count := 0
	for _, f := range p.Familiars {
		if f.Unlocked {
			count++
		}
	}
	return count
}
