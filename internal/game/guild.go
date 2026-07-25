package game

import (
	"sync"
	"time"
)

type GuildRank struct {
	ID      uint32
	Name    string
	Level   uint8
	GuildID uint32
}

type Guild struct {
	mu sync.RWMutex

	ID           uint32
	Name         string
	OwnerID      uint32
	CreationDate time.Time
	MOTD         string
	Balance      uint64
	Points       int32
	Level        int32

	Ranks         []*GuildRank
	MembersOnline []*Player
	MemberCount   uint32
}

func NewGuild(id uint32, name string) *Guild {
	return &Guild{
		ID:            id,
		Name:          name,
		Ranks:         make([]*GuildRank, 0),
		MembersOnline: make([]*Player, 0),
	}
}

func (g *Guild) AddMember(p *Player) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, member := range g.MembersOnline {
		if member.ID == p.ID {
			return
		}
	}
	g.MembersOnline = append(g.MembersOnline, p)
}

func (g *Guild) RemoveMember(p *Player) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for i, member := range g.MembersOnline {
		if member.ID == p.ID {
			g.MembersOnline = append(g.MembersOnline[:i], g.MembersOnline[i+1:]...)
			return
		}
	}
}

func (g *Guild) GetMembersOnline() []*Player {
	g.mu.RLock()
	defer g.mu.RUnlock()

	members := make([]*Player, len(g.MembersOnline))
	copy(members, g.MembersOnline)
	return members
}

func (g *Guild) GetMemberCountOnline() uint32 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return uint32(len(g.MembersOnline))
}

func (g *Guild) AddRank(id uint32, name string, level uint8) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.Ranks = append(g.Ranks, &GuildRank{
		ID:      id,
		Name:    name,
		Level:   level,
		GuildID: g.ID,
	})
}

func (g *Guild) GetRankByID(id uint32) *GuildRank {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, rank := range g.Ranks {
		if rank.ID == id {
			return rank
		}
	}
	return nil
}

func (g *Guild) GetRankByName(name string) *GuildRank {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, rank := range g.Ranks {
		if rank.Name == name {
			return rank
		}
	}
	return nil
}

func (g *Guild) GetRankByLevel(level uint8) *GuildRank {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, rank := range g.Ranks {
		if rank.Level == level {
			return rank
		}
	}
	return nil
}

func (g *Guild) GetRanks() []*GuildRank {
	g.mu.RLock()
	defer g.mu.RUnlock()

	ranks := make([]*GuildRank, len(g.Ranks))
	copy(ranks, g.Ranks)
	return ranks
}

func (g *Guild) GetMOTD() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.MOTD
}

func (g *Guild) SetMOTD(motd string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.MOTD = motd
}

func (g *Guild) GetBankBalance() uint64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.Balance
}

func (g *Guild) SetBankBalance(balance uint64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Balance = balance
}

type GuildMember struct {
	PlayerID uint32
	GuildID  uint32
	RankID   uint32
	Nick     string
}
