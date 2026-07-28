package game

// ---- Team Finder stubs (Batch 1) ----

// PlayerFindTeam looks for a team matching the given criteria.
func (w *World) PlayerFindTeam(playerID uint32, slot int, minLevel int, maxLevel int, vocation int, description int) {
	// No-op stub.
}

// RemoveTeamFinder removes the player from the team finder.
func (w *World) RemoveTeamFinder(playerID uint32) {
	// No-op stub.
}

// UpdateTeamMemberStatus updates the status of a member in the leader's team finder.
func (w *World) UpdateTeamMemberStatus(leaderID uint32, memberID uint32, status byte) {
	// No-op stub.
}

// SendTeamFinderList sends the team finder list to the player.
func (w *World) SendTeamFinderList(playerID uint32) {
	// No-op stub.
}

// JoinTeamFinder adds the player to the leader's team finder.
func (w *World) JoinTeamFinder(playerID uint32, leaderID uint32) {
	// No-op stub.
}

// LeaveTeamFinder removes the player from the leader's team finder.
func (w *World) LeaveTeamFinder(playerID uint32, leaderID uint32) {
	// No-op stub.
}

// ---- Action stubs (Batch 1) ----

// PlayerSetVocation sets the vocation of the player (Dawnport).
func (w *World) PlayerSetVocation(playerID uint32, vocationID byte) {
	// No-op stub.
}

// PlayerTeleport teleports the player to the given position (GM command).
func (w *World) PlayerTeleport(playerID uint32, pos Position) {
	// No-op stub.
}

// PlayerRequestTrade initiates a trade between two players.
func (w *World) PlayerRequestTrade(playerID uint32, targetPlayerID uint32, pos Position, itemID uint16, stackPos byte) {
	// No-op stub.
}

// PlayerAcceptTrade accepts the current trade.
func (w *World) PlayerAcceptTrade(playerID uint32) {
	// No-op stub.
}

// PlayerCloseTrade closes/cancels the current trade.
func (w *World) PlayerCloseTrade(playerID uint32) {
	// No-op stub.
}

// PlayerRotateItem rotates an item on the map.
func (w *World) PlayerRotateItem(playerID uint32, pos Position, itemID uint16, stackPos byte) {
	// No-op stub.
}

// PlayerJoinAggression joins aggression against a target.
func (w *World) PlayerJoinAggression(playerID uint32, targetID uint32) {
	// No-op stub.
}

// PlayerCloseNpcChannel closes the NPC trade channel for the player.
func (w *World) PlayerCloseNpcChannel(playerID uint32) {
	// No-op stub.
}

// PlayerFollow makes the player follow a target creature.
func (w *World) PlayerFollow(playerID uint32, targetID uint32) {
	// No-op stub.
}
