package game

// OpenContainer mirrors C++ struct OpenContainer { Container* container; uint16_t index }.
// Index is the pagination first-index (scroll offset) for the open window.
type OpenContainer struct {
	Container *Item
	Index     uint16
	Position  Position
	IsOnMap   bool
}

// maxOpenContainers is the client cap on simultaneously open container windows
// (cids 0..15). Mirrors the 0..0xF range used by the protocol.
const maxOpenContainers = 16

// GetContainerID returns the client container id the given container is open
// under, or -1 when it is not open. Mirrors Player::getContainerID (returns -1,
// never 0, because cid 0 is a valid open window).
func (p *Player) GetContainerID(c *Item) int {
	for cid, oc := range p.openContainers {
		if oc.Container == c {
			return int(cid)
		}
	}
	return -1
}

// GetContainerByID returns the container open under the given client id, or nil.
func (p *Player) GetContainerByID(cid uint8) *Item {
	if p.openContainers == nil {
		return nil
	}
	return p.openContainers[cid].Container
}

// GetContainerIndex returns the pagination scroll index for the given open cid.
func (p *Player) GetContainerIndex(cid uint8) uint16 {
	if p.openContainers == nil {
		return 0
	}
	return p.openContainers[cid].Index
}

// SetContainerIndex updates the pagination scroll index for an open cid.
func (p *Player) SetContainerIndex(cid uint8, index uint16) {
	if p.openContainers == nil {
		return
	}
	if oc, ok := p.openContainers[cid]; ok {
		oc.Index = index
		p.openContainers[cid] = oc
	}
}

// AddContainer registers a container as open, reusing its existing cid when it
// is already open, else allocating the lowest free cid (0..15). Returns the cid,
// or -1 when all slots are taken. Mirrors Player::addContainer.
func (p *Player) AddContainer(c *Item) int {
	return p.AddContainerWithPos(c, Position{}, false)
}

// AddContainerWithPos registers a container as open, with explicit position / IsOnMap metadata.
func (p *Player) AddContainerWithPos(c *Item, pos Position, isOnMap bool) int {
	if c == nil {
		return -1
	}
	if p.openContainers == nil {
		p.openContainers = make(map[uint8]OpenContainer)
	}
	if cid := p.GetContainerID(c); cid != -1 {
		p.openContainers[uint8(cid)] = OpenContainer{Container: c, Position: pos, IsOnMap: isOnMap}
		return cid
	}
	for cid := 0; cid < maxOpenContainers; cid++ {
		if _, taken := p.openContainers[uint8(cid)]; !taken {
			p.openContainers[uint8(cid)] = OpenContainer{Container: c, Position: pos, IsOnMap: isOnMap}
			return cid
		}
	}
	return -1
}

// OpenContainerAt registers c as open under an explicit client cid, preserving
// any existing pagination index. Used when the client requests a container be
// (re)opened in a specific window.
func (p *Player) OpenContainerAt(cid uint8, c *Item) {
	p.OpenContainerAtWithPos(cid, c, Position{}, false)
}

// OpenContainerAtWithPos registers c as open under an explicit client cid, with explicit position / IsOnMap metadata.
// C++: reseta o scroll index ao abrir um container diferente (evita que
// o Index paginado do container anterior seja herdado pelo novo).
func (p *Player) OpenContainerAtWithPos(cid uint8, c *Item, pos Position, isOnMap bool) {
	if p.openContainers == nil {
		p.openContainers = make(map[uint8]OpenContainer)
	}
	oc := p.openContainers[cid]
	if oc.Container != c {
		oc.Index = 0
	}
	oc.Container = c
	oc.Position = pos
	oc.IsOnMap = isOnMap
	p.openContainers[cid] = oc
}

// CloseContainer removes the open-container entry for a cid. Mirrors
// Player::closeContainer.
func (p *Player) CloseContainer(cid uint8) {
	if p.openContainers != nil {
		delete(p.openContainers, cid)
	}
}

// OpenContainersSnapshot returns a copy of the open-container map for iteration
// without exposing the internal map to mutation.
func (p *Player) OpenContainersSnapshot() map[uint8]OpenContainer {
	out := make(map[uint8]OpenContainer, len(p.openContainers))
	for k, v := range p.openContainers {
		out[k] = v
	}
	return out
}
