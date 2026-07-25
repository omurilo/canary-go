package game

func (w *World) LoadGuild(guildID uint32) *Guild {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.guilds[guildID]
}

func (w *World) RegisterGuild(guild *Guild) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.guilds == nil {
		w.guilds = make(map[uint32]*Guild)
	}
	w.guilds[guild.ID] = guild
}

func (w *World) UnregisterGuild(guildID uint32) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.guilds, guildID)
}

func (w *World) GetGuild(guildID uint32) *Guild {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.guilds[guildID]
}

func (p *Player) GetGuild() *Guild {
	if p.World == nil {
		return nil
	}
	
	if p.GuildName == "" {
		return nil
	}
	
	w := p.World
	w.mu.RLock()
	defer w.mu.RUnlock()
	
	for _, guild := range w.guilds {
		if guild.Name == p.GuildName {
			return guild
		}
	}
	
	return nil
}

func (p *Player) SetGuildNick(nick string) {
	p.GuildNick = nick
}

func (p *Player) GetGuildNick() string {
	return p.GuildNick
}

func (p *Player) GetGuildLevel() uint8 {
	guild := p.GetGuild()
	if guild == nil {
		return 0
	}
	
	for _, rank := range guild.Ranks {
		if rank.Name == p.GuildRankName {
			return rank.Level
		}
	}
	return 0
}

func (p *Player) SetGuildLevel(level uint8) {
}
