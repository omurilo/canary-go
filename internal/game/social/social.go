package social

import (
	"sync"
)

// VI PEntry represents a VIP entry
type VIPEntry struct {
	PlayerID uint32
	Name     string
}

// SocialManager handles party, guild and vip states
type SocialManager struct {
	mu        sync.RWMutex
	VIPList   map[uint32]*VIPEntry
	GuildID   uint32
	GuildRank uint32
	Party     interface{} // Ref to player.Party
}

func NewSocialManager() *SocialManager {
	return &SocialManager{
		VIPList: make(map[uint32]*VIPEntry),
	}
}
