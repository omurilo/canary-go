package protocol

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// Effect and message constants (utils_definitions.hpp / server_definitions.hpp).
const (
	constMETeleport     = 11 // CONST_ME_TELEPORT
	messageStatus       = 30 // MESSAGE_STATUS (white text at the bottom + console)
	magicEffectsCreate  = 3  // MAGIC_EFFECTS_CREATE_EFFECT
	magicEffectsEndLoop = 0  // MAGIC_EFFECTS_END_LOOP
	sourceEffectOwn     = 1  // SourceEffect_t::OWN
)

// sendMagicEffect shows a graphical effect at pos, mirroring the modern layout of
// ProtocolGame::sendMagicEffect: create-effect, u16 type, source byte, end-loop.
func (g *GameProtocol) sendMagicEffect(pos game.Position, effect uint16) {
	w := netmsg.NewWriter()
	w.AddByte(opMagicEffect)
	w.AddPosition(netmsg.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
	w.AddByte(magicEffectsCreate)
	w.AddU16(effect)
	w.AddByte(sourceEffectOwn)
	w.AddByte(magicEffectsEndLoop)
	g.SendToClient(w)
}

// sendStatusText shows a white status message (0xB4 MESSAGE_STATUS).
func (g *GameProtocol) sendStatusText(text string) {
	w := netmsg.NewWriter()
	w.AddByte(opTextMessage)
	w.AddByte(messageStatus)
	w.AddString(text)
	g.SendToClient(w)
}

// teleport moves the player to an absolute position and resyncs the client view,
// mirroring the teleport branch of ProtocolGame::sendMoveCreature (remove from the
// old tile, then a full map description at the destination).
func (g *GameProtocol) teleport(dest game.Position) {
	g.actionMu.Lock()
	defer g.actionMu.Unlock()
	g.walkGen.Add(1) // cancel any auto-walk in flight

	p := g.player
	oldPos := p.Pos

	oldStack := g.StackPosOf(oldPos, p.ID)

	g.broadcastRemove(p) // old spectators see us vanish
	g.deps.World.SetPosition(p, dest)

	g.SendRemoveCreatureAt(oldPos, oldStack) // self: drop the old marker
	
	w := netmsg.NewWriter()
	w.AddByte(opFullMap)
	w.AddPosition(netmsg.Position{X: dest.X, Y: dest.Y, Z: dest.Z})
	g.addMapDescription(w, int(dest.X)-viewportX, int(dest.Y)-viewportY, dest.Z, mapWidth, mapHeight)
	g.SendToClient(w)

	g.sendMagicEffect(dest, constMETeleport)
	g.broadcastAppear(p) // new spectators see us appear
}

// handleCommand runs a leading-slash GM command. Returns true if text was handled
// as a command (and therefore should not be broadcast as normal chat).
func (g *GameProtocol) handleCommand(text string) bool {
	if !strings.HasPrefix(text, "/") {
		return false
	}
	fields := strings.Fields(text[1:])
	if len(fields) == 0 {
		return false
	}
	cmd := strings.ToLower(fields[0])
	args := fields[1:]
	p := g.player

	// All native commands require gamemaster+
	if !hasGroupPermission(p, "gamemaster") {
		g.sendStatusText("You cannot execute this command.")
		return true
	}

	switch cmd {
	case "pos", "position":
		g.sendStatusText(fmt.Sprintf("Position: [%d, %d, %d]", p.Pos.X, p.Pos.Y, p.Pos.Z))
	case "up":
		if p.AccountType < 5 {
			g.sendStatusText("You don't have access to this command.")
			return true
		}
		g.floorTeleport(-1)
	case "down":
		if p.AccountType < 5 {
			g.sendStatusText("You don't have access to this command.")
			return true
		}
		g.floorTeleport(+1)
	case "goto", "go", "cliport":
		if p.AccountType < 5 {
			g.sendStatusText("You don't have access to this command.")
			return true
		}
		g.cmdGoto(args)
	case "town", "t":
		if p.AccountType < 5 {
			g.sendStatusText("You don't have access to this command.")
			return true
		}
		g.cmdTown(args)
	case "i", "create":
		if p.AccountType < 5 {
			g.sendStatusText("You don't have access to this command.")
			return true
		}
		g.cmdCreateItem(args)
	case "addskill":
		g.cmdAddSkill(args)
	case "save":
		if p.AccountType < 5 {
			g.sendStatusText("You don't have access to this command.")
			return true
		}
		g.cmdSave()
	case "b", "broadcast":
		if p.AccountType < 5 {
			g.sendStatusText("You don't have access to this command.")
			return true
		}
		g.cmdBroadcast(args)
	case "outfit":
		g.SendOutfitWindow()
	case "addexp":
		if p.AccountType < 5 {
			g.sendStatusText("You don't have access to this command.")
			return true
		}
		g.cmdAddExp(args)
	case "addmoney":
		if p.AccountType < 5 {
			g.sendStatusText("You don't have access to this command.")
			return true
		}
		g.cmdAddMoney(args)
	case "level":
		if p.AccountType < 5 {
			g.sendStatusText("You don't have access to this command.")
			return true
		}
		g.cmdSetLevel(args)
	case "health", "hp":
		g.cmdSetHealth(args)
	case "mana", "mp":
		g.cmdSetMana(args)
	case "speed":
		g.cmdSetSpeed(args)
	case "online":
		g.cmdOnline()
	case "info":
		g.cmdInfo()
	case "skull":
		if p.AccountType < 5 {
			g.sendStatusText("You don't have access to this command.")
			return true
		}
		g.cmdSkull(args)
	case "sex", "gender":
		g.cmdToggleSex()
	case "summon":
		if p.AccountType < 5 {
			g.sendStatusText("You don't have access to this command.")
			return true
		}
		g.cmdSummon(args)
	case "kick":
		if p.AccountType < 5 {
			g.sendStatusText("You don't have access to this command.")
			return true
		}
		g.cmdKick(args)
	case "commands", "help":
		g.sendStatusText("Commands: /pos /goto /up /down /town /i /addexp /addmoney /level /health /mana /speed /online /info /skull /sex /summon /kick /b /save /addskill /outfit")
	default:
		return false // Let Lua talkactions try
	}
	return true
}

// targetPlayerFromArgs looks for "PlayerName, <rest>" in args.
func targetPlayerFromArgs(g *GameProtocol, args []string) (*game.Player, []string, bool) {
	full := strings.TrimSpace(strings.Join(args, " "))
	if idx := strings.LastIndex(full, ","); idx > 0 {
		name := strings.TrimSpace(full[:idx])
		if p := g.deps.World.PlayerByName(name); p != nil {
			rest := strings.TrimSpace(full[idx+1:])
			if rest == "" {
				return p, nil, true
			}
			return p, strings.Fields(rest), true
		}
	}
	return nil, args, false
}

// cmdCreateItem places an item on the tile under the player (/i <id> [count]).
func (g *GameProtocol) cmdCreateItem(args []string) {
	parts := strings.Fields(strings.ReplaceAll(strings.Join(args, " "), ",", " "))
	if len(parts) == 0 {
		g.sendStatusText("Usage: /i <id> [count]")
		return
	}
	id, err := strconv.Atoi(parts[0])
	if err != nil || id < 100 || id > 0xFFFF || g.deps.Items.Get(uint16(id)) == nil {
		g.sendStatusText("There is no item with that id.")
		return
	}
	count := 1
	if len(parts) >= 2 {
		if c, e := strconv.Atoi(parts[1]); e == nil && c > 0 {
			count = c
		}
	}
	if count > 100 {
		count = 100
	}
	pos := g.player.Pos
	item := &game.Item{ID: uint16(id), Count: uint16(count)}
	if !g.deps.World.AddItem(pos, item) {
		g.sendStatusText("No tile to place the item on.")
		return
	}
	g.broadcastAddItem(pos, item)
	g.sendStatusText(fmt.Sprintf("Created item %d (x%d).", id, count))
}

// broadcastAddItem tells the player and nearby spectators an item appeared on top
// of a tile (0x6A TileAddThing with the item's stack index).
func (g *GameProtocol) broadcastAddItem(pos game.Position, item *game.Item) {
	send := func(gp *GameProtocol) {
		// The new item sits on top of everything currently on the tile.

		stack := 0
		if t := gp.deps.World.Map.GetTile(pos); t != nil {
			if t.Ground != nil {
				stack++
			}
			stack += len(t.Items) - 1 // the item is already in the stack; index is the rest
			stack += len(t.Creatures)
		}
		w := netmsg.NewWriter()
		w.AddByte(0x6A) // TileAddThing
		w.AddPosition(netmsg.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
		w.AddByte(byte(stack - 1))
		gp.addItem(w, item)
		gp.SendToClient(w)
	}
	send(g)
	for _, s := range g.deps.World.Spectators(pos, g.player.ID) {
		if gp, ok := s.Session.(*GameProtocol); ok && gp.isKnown(g.player.ID) {
			gp.SendRemoveCreatureAt(pos, 0)
		}
	}
}

// cmdAddSkill raises a skill: /addskill <fist|club|sword|axe|dist|shield|fish|ml> [n].
func (g *GameProtocol) cmdAddSkill(args []string) {
	if len(args) == 0 {
		g.sendStatusText("Usage: /addskill <fist|club|sword|axe|dist|shield|fish|ml> [n]")
		return
	}
	n := 1
	if len(args) >= 2 {
		if v, e := strconv.Atoi(args[1]); e == nil && v > 0 {
			n = v
		}
	}
	p := g.player
	switch strings.ToLower(args[0]) {
	case "fist":
		p.Skills[game.SkillFist] += uint16(n)
	case "club":
		p.Skills[game.SkillClub] += uint16(n)
	case "sword":
		p.Skills[game.SkillSword] += uint16(n)
	case "axe":
		p.Skills[game.SkillAxe] += uint16(n)
	case "dist", "distance":
		p.Skills[game.SkillDistance] += uint16(n)
	case "shield", "shielding":
		p.Skills[game.SkillShielding] += uint16(n)
	case "fish", "fishing":
		p.Skills[game.SkillFishing] += uint16(n)
	case "ml", "magic":
		p.MagLevel += uint16(n)
	default:
		g.sendStatusText("Unknown skill: " + args[0])
		return
	}
	w := netmsg.NewWriter()
	g.addSkills(w)
	g.SendToClient(w)
	g.sendStatusText("Skill updated.")
}

// cmdSave persists every online player.
func (g *GameProtocol) cmdSave() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	saved := 0
	for _, p := range g.deps.World.Players() {
		if err := g.deps.DB.SavePlayer(ctx, p); err == nil {
			saved++
		}
	}
	g.sendStatusText(fmt.Sprintf("Saved %d player(s).", saved))
}

// cmdBroadcast shows a status message to every online player (/b <text>).
func (g *GameProtocol) cmdBroadcast(args []string) {
	text := strings.TrimSpace(strings.Join(args, " "))
	if text == "" {
		return
	}
	msg := g.player.Name + ": " + text
	for _, p := range g.deps.World.Players() {
		if gp, ok := p.Session.(*GameProtocol); ok {
			gp.sendStatusText(msg)
		}
	}
}

// floorTeleport moves the player one floor up (dz=-1) or down (dz=+1).
func (g *GameProtocol) floorTeleport(dz int) {
	nz := int(g.player.Pos.Z) + dz
	if nz < 0 || nz > 15 {
		g.sendStatusText("Cannot go further.")
		return
	}
	g.teleport(game.Position{X: g.player.Pos.X, Y: g.player.Pos.Y, Z: uint8(nz)})
}

// cmdGoto teleports to explicit coordinates: /goto <x> <y> [z] (commas allowed).
func (g *GameProtocol) cmdGoto(args []string) {
	parts := strings.Fields(strings.ReplaceAll(strings.Join(args, " "), ",", " "))
	if len(parts) < 2 {
		g.sendStatusText("Usage: /goto <x> <y> [z]")
		return
	}
	x, err1 := strconv.Atoi(parts[0])
	y, err2 := strconv.Atoi(parts[1])
	z := int(g.player.Pos.Z)
	if len(parts) >= 3 {
		if zz, err := strconv.Atoi(parts[2]); err == nil {
			z = zz
		}
	}
	if err1 != nil || err2 != nil || x < 0 || x > 0xFFFF || y < 0 || y > 0xFFFF || z < 0 || z > 15 {
		g.sendStatusText("Invalid coordinates.")
		return
	}
	g.teleport(game.Position{X: uint16(x), Y: uint16(y), Z: uint8(z)})
}

// cmdTown teleports to a town temple: /town <name> (no arg lists the towns).
func (g *GameProtocol) cmdTown(args []string) {
	towns := g.deps.World.Towns
	if len(args) == 0 {
		names := make([]string, 0, len(towns))
		for name := range towns {
			names = append(names, name)
		}
		sort.Strings(names)
		g.sendStatusText("Towns: " + strings.Join(names, ", "))
		return
	}
	name := strings.Join(args, " ")
	if pos, ok := g.deps.World.TownTemple(name); ok {
		g.teleport(pos)
		g.sendStatusText(fmt.Sprintf("Teleported to %s [%d, %d, %d].", name, pos.X, pos.Y, pos.Z))
	} else {
		g.sendStatusText("Unknown town: " + name)
	}
}

// cmdAddExp adds experience: /addexp <amount> | /addexp PlayerName, <amount>
func (g *GameProtocol) cmdAddExp(args []string) {
	target, rest, hasTarget := targetPlayerFromArgs(g, args)
	if !hasTarget {
		target = g.player
		rest = args
	}
	if len(rest) == 0 {
		g.sendStatusText("Usage: /addexp <amount> or /addexp PlayerName, <amount>")
		return
	}
	amount, err := strconv.ParseUint(rest[0], 10, 64)
	if err != nil || amount == 0 {
		g.sendStatusText("Invalid amount.")
		return
	}
	target.AddExperience(amount)
	g.sendStatusText(fmt.Sprintf("Added %d experience to %s.", amount, target.Name))
}

// cmdAddMoney adds gold to bank: /addmoney <amount>
func (g *GameProtocol) cmdAddMoney(args []string) {
	if len(args) == 0 {
		g.sendStatusText("Usage: /addmoney <amount>")
		return
	}
	amount, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil || amount == 0 {
		g.sendStatusText("Invalid amount.")
		return
	}
	g.player.BankBalance += amount
	g.sendResourceBalances()
	g.sendStatusText(fmt.Sprintf("Added %d gold to bank.", amount))
}

// cmdSetLevel sets player level: /level <n>
func (g *GameProtocol) cmdSetLevel(args []string) {
	if len(args) == 0 {
		g.sendStatusText(fmt.Sprintf("Current level: %d", g.player.Level))
		return
	}
	lvl, err := strconv.ParseUint(args[0], 10, 16)
	if err != nil || lvl == 0 || lvl > 2000 {
		g.sendStatusText("Invalid level (1-2000).")
		return
	}
	p := g.player
	p.Level = uint16(lvl)
	p.Experience = game.ExpForLevel(uint64(lvl))
	if p.Session != nil {
		p.Session.SendStats()
	}
	g.sendStatusText(fmt.Sprintf("Set level to %d.", lvl))
}

// cmdSetHealth sets/maxes health: /health [hp]
func (g *GameProtocol) cmdSetHealth(args []string) {
	p := g.player
	if len(args) == 0 {
		p.Health = p.MaxHealth
	} else {
		hp, err := strconv.Atoi(args[0])
		if err != nil || hp < 1 {
			g.sendStatusText("Invalid HP.")
			return
		}
		p.Health = uint32(hp)
	}
	if p.Session != nil {
		p.Session.SendStats()
	}
	g.sendStatusText(fmt.Sprintf("Health set to %d.", p.Health))
}

// cmdSetMana sets/maxes mana: /mana [mp]
func (g *GameProtocol) cmdSetMana(args []string) {
	p := g.player
	if len(args) == 0 {
		p.Mana = p.MaxMana
	} else {
		mp, err := strconv.Atoi(args[0])
		if err != nil || mp < 1 {
			g.sendStatusText("Invalid MP.")
			return
		}
		p.Mana = uint32(mp)
	}
	if p.Session != nil {
		p.Session.SendStats()
	}
	g.sendStatusText(fmt.Sprintf("Mana set to %d.", p.Mana))
}

// cmdSetSpeed sets movement speed: /speed <value>
func (g *GameProtocol) cmdSetSpeed(args []string) {
	if len(args) == 0 {
		g.sendStatusText(fmt.Sprintf("Current speed: %d", g.player.Speed))
		return
	}
	spd, err := strconv.ParseUint(args[0], 10, 16)
	if err != nil || spd == 0 || spd > 100000 {
		g.sendStatusText("Invalid speed.")
		return
	}
	g.player.Speed = uint16(spd)
	if g.player.Session != nil {
		g.player.Session.SendChangeSpeed(g.player)
	}
	g.sendStatusText(fmt.Sprintf("Speed set to %d.", spd))
}

// cmdOnline lists online players.
func (g *GameProtocol) cmdOnline() {
	players := g.deps.World.Players()
	names := make([]string, 0, len(players))
	for _, p := range players {
		if p != nil {
			names = append(names, p.Name)
		}
	}
	g.sendStatusText(fmt.Sprintf("Online (%d): %s", len(names), strings.Join(names, ", ")))
}

// cmdInfo shows player info.
func (g *GameProtocol) cmdInfo() {
	p := g.player
	g.sendStatusText(fmt.Sprintf("Name: %s | Level: %d | Vocation: %d | HP: %d/%d | MP: %d/%d | Bank: %d",
		p.Name, p.Level, p.Vocation, p.Health, p.MaxHealth, p.Mana, p.MaxMana, p.BankBalance))
}

// cmdSkull sets skull type: /skull <0-5>
func (g *GameProtocol) cmdSkull(args []string) {
	if len(args) == 0 {
		g.sendStatusText("Usage: /skull <0=none 1=yellow 2=green 3=white 4=red 5=black>")
		return
	}
	sk, err := strconv.Atoi(args[0])
	if err != nil || sk < 0 || sk > 5 {
		g.sendStatusText("Invalid skull type (0-5).")
		return
	}
	g.player.Skull = uint8(sk)
	g.sendStatusText(fmt.Sprintf("Skull set to %d.", sk))
}

// cmdToggleSex toggles the player's gender.
func (g *GameProtocol) cmdToggleSex() {
	p := g.player
	if p.Sex == 0 {
		p.Sex = 1
	} else {
		p.Sex = 0
	}
	g.sendStatusText(fmt.Sprintf("Sex changed to %s.", map[uint8]string{0: "female", 1: "male"}[p.Sex]))
	// Update outfit to reflect new sex
	g.SendOutfitWindow()
}

// cmdSummon shows usage info: full summon is handled by Lua talkaction /c
func (g *GameProtocol) cmdSummon(args []string) {
	g.sendStatusText("Use /c <name> to summon creatures.")
}

// cmdKick kicks a player: /kick <player>
func (g *GameProtocol) cmdKick(args []string) {
	if len(args) == 0 {
		g.sendStatusText("Usage: /kick <player>")
		return
	}
	name := strings.Join(args, " ")
	if p := g.deps.World.PlayerByName(name); p != nil {
		if p.Session != nil {
			p.Session.Disconnect()
		}
		g.sendStatusText(fmt.Sprintf("Kicked %s.", name))
	} else {
		g.sendStatusText("Player not found: " + name)
	}
}
