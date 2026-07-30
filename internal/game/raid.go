package game

import (
	"context"
	"encoding/xml"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// RaidState tracks a raid's lifecycle.
type RaidState int

const (
	RaidStateInactive  RaidState = iota
	RaidStateRunning
	RaidStateCompleted
)

// RaidEvent is one step in a raid event sequence.
type RaidEvent interface {
	Delay() time.Duration
	Execute(w *World) error
}

// AnnounceEvent broadcasts a message to all online players.
type AnnounceEvent struct {
	DelayMs int    `xml:"delay,attr"`
	Type    string `xml:"type,attr"`
	Message string `xml:"message,attr"`
}

func (e *AnnounceEvent) Delay() time.Duration {
	return time.Duration(e.DelayMs) * time.Millisecond
}

func (e *AnnounceEvent) Execute(w *World) error {
	for _, p := range w.Players() {
		if p.Session != nil {
			m := netmsg.NewWriter()
			m.AddByte(0xB4) // opTextMessage
			m.AddByte(byte(0x10)) // MESSAGE_EVENT_ADVANCE
			m.AddString(e.Message)
			p.Session.SendToClient(m)
		}
	}
	return nil
}

// SingleSpawnEvent spawns one monster at a specific position.
type SingleSpawnEvent struct {
	DelayMs int    `xml:"delay,attr"`
	Name    string `xml:"name,attr"`
	X       int    `xml:"x,attr"`
	Y       int    `xml:"y,attr"`
	Z       int    `xml:"z,attr"`
}

func (e *SingleSpawnEvent) Delay() time.Duration {
	return time.Duration(e.DelayMs) * time.Millisecond
}

func (e *SingleSpawnEvent) Execute(w *World) error {
	pos := Position{X: uint16(e.X), Y: uint16(e.Y), Z: uint8(e.Z)}
	if w.Map.GetTile(pos) == nil {
		return fmt.Errorf("raid singlespawn: invalid position %v for %s", pos, e.Name)
	}
	mType := w.MonsterType(e.Name)
	monster := NewMonster(w.GenerateCreatureID(), e.Name, mType)
	monster.SetPosition(pos)
	w.AddCreature(monster)
	return nil
}

// AreaSpawnEvent spawns multiple monsters within a rectangular area.
type AreaSpawnEvent struct {
	DelayMs  int           `xml:"delay,attr"`
	FromX    int           `xml:"fromx,attr"`
	FromY    int           `xml:"fromy,attr"`
	FromZ    int           `xml:"fromz,attr"`
	ToX      int           `xml:"tox,attr"`
	ToY      int           `xml:"toy,attr"`
	ToZ      int           `xml:"toz,attr"`
	Monsters []AreaMonster `xml:"monster"`
}

type AreaMonster struct {
	Name   string `xml:"name,attr"`
	Amount int    `xml:"amount,attr"`
}

func (e *AreaSpawnEvent) Delay() time.Duration {
	return time.Duration(e.DelayMs) * time.Millisecond
}

func (e *AreaSpawnEvent) Execute(w *World) error {
	for _, monster := range e.Monsters {
		for i := 0; i < monster.Amount; i++ {
			x := e.FromX + rand.Intn(e.ToX-e.FromX+1)
			y := e.FromY + rand.Intn(e.ToY-e.FromY+1)
			pos := Position{X: uint16(x), Y: uint16(y), Z: uint8(e.FromZ)}
			tile := w.Map.GetTile(pos)
			if tile == nil || !tile.WalkableFor(nil, w.Items, w.WorldType) {
				continue
			}
			mType := w.MonsterType(monster.Name)
			m := NewMonster(w.GenerateCreatureID(), monster.Name, mType)
			m.SetPosition(pos)
			w.AddCreature(m)
		}
	}
	return nil
}

// ScriptEvent invokes a Lua function (stub for Phase 3).
type ScriptEvent struct {
	DelayMs int    `xml:"delay,attr"`
	Script  string `xml:"script,attr"`
}

func (e *ScriptEvent) Delay() time.Duration {
	return time.Duration(e.DelayMs) * time.Millisecond
}

func (e *ScriptEvent) Execute(w *World) error {
	slog.Warn("raid: ScriptEvent not implemented (stub)", "script", e.Script)
	return nil
}

// Raid represents a single raid definition from the XML.
type Raid struct {
	Name     string
	Interval time.Duration
	Margin   time.Duration
	State    RaidState
	Events   []RaidEvent

	mu             sync.Mutex
	lastOccurrence time.Time
	nextOccurrence time.Time
}

// Start begins executing a raid's event sequence via the global dispatcher.
func (r *Raid) Start(w *World) {
	r.mu.Lock()
	if r.State != RaidStateInactive {
		r.mu.Unlock()
		return
	}
	r.State = RaidStateRunning
	r.mu.Unlock()

	for _, event := range r.Events {
		ev := event
		delay := ev.Delay()
		GlobalDispatcher.AddEvent(delay, func() {
			if err := ev.Execute(w); err != nil {
				slog.Error("raid event error", "raid", r.Name, "err", err)
			}
		})
	}

	// Schedule next occurrence.
	r.mu.Lock()
	r.State = RaidStateCompleted
	r.lastOccurrence = time.Now()
	r.scheduleNext()
	r.mu.Unlock()
}

func (r *Raid) scheduleNext() {
	if r.Interval <= 0 {
		return
	}
	jitter := time.Duration(rand.Int63n(int64(r.Margin))) - (r.Margin / 2)
	r.nextOccurrence = time.Now().Add(r.Interval + jitter)
}

// Raids is the global raid manager.
type Raids struct {
	mu      sync.Mutex
	raids   map[string]*Raid
	ticker  *time.Ticker
	running bool
}

// LoadRaids parses raids.xml and all referenced raid definition files.
func LoadRaids(path string) (*Raids, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("raids: read %s: %w", path, err)
	}
	var xmlDef raidsXML
	if err := xml.Unmarshal(data, &xmlDef); err != nil {
		return nil, fmt.Errorf("raids: unmarshal %s: %w", path, err)
	}
	base := filepath.Dir(path)

	rs := &Raids{
		raids: make(map[string]*Raid),
	}
	for _, def := range xmlDef.Raids {
		raidDefPath := filepath.Join(base, def.File)
		raid, err := loadRaidDefinition(raidDefPath, def)
		if err != nil {
			slog.Warn("raids: failed to load raid definition", "name", def.Name, "path", raidDefPath, "err", err)
			continue
		}
		rs.raids[strings.ToLower(def.Name)] = raid
	}
	slog.Info("raids loaded", "count", len(rs.raids))
	return rs, nil
}

// raidsXML is the root structure of raids.xml.
type raidsXML struct {
	XMLName xml.Name  `xml:"raids"`
	Raids   []raidDef `xml:"raid"`
}

type raidDef struct {
	Name      string `xml:"name,attr"`
	File      string `xml:"file,attr"`
	Interval2 int    `xml:"interval2,attr"`
	Margin    int    `xml:"margin,attr"`
}

// raidDefXML is the structure of an individual raid definition XML file.
type raidDefXML struct {
	XMLName     xml.Name           `xml:"raid"`
	Announce    []AnnounceEvent    `xml:"announce"`
	SingleSpawn []SingleSpawnEvent `xml:"singlespawn"`
	AreaSpawn   []AreaSpawnEvent   `xml:"areaspawn"`
	Script      []ScriptEvent      `xml:"script"`
}

func loadRaidDefinition(path string, def raidDef) (*Raid, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var xmlEvents raidDefXML
	if err := xml.Unmarshal(data, &xmlEvents); err != nil {
		return nil, fmt.Errorf("raid def: unmarshal %s: %w", path, err)
	}
	raid := &Raid{
		Name:     def.Name,
		Interval: time.Duration(def.Interval2) * time.Minute,
		Margin:   time.Duration(def.Margin) * time.Minute,
		State:    RaidStateInactive,
	}
	for i := range xmlEvents.Announce {
		raid.Events = append(raid.Events, &xmlEvents.Announce[i])
	}
	for i := range xmlEvents.AreaSpawn {
		raid.Events = append(raid.Events, &xmlEvents.AreaSpawn[i])
	}
	for i := range xmlEvents.SingleSpawn {
		raid.Events = append(raid.Events, &xmlEvents.SingleSpawn[i])
	}
	for i := range xmlEvents.Script {
		raid.Events = append(raid.Events, &xmlEvents.Script[i])
	}
	return raid, nil
}

// Start begins the background raid scheduler goroutine.
func (rs *Raids) Start(ctx context.Context, w *World) {
	rs.mu.Lock()
	if rs.running {
		rs.mu.Unlock()
		return
	}
	rs.running = true
	rs.mu.Unlock()

	for _, raid := range rs.raids {
		raid.mu.Lock()
		raid.scheduleNext()
		raid.mu.Unlock()
	}

	rs.ticker = time.NewTicker(60 * time.Second)
	go func() {
		defer rs.ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-rs.ticker.C:
				rs.checkRaids(w)
			}
		}
	}()
}

func (rs *Raids) checkRaids(w *World) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	now := time.Now()
	for _, raid := range rs.raids {
		raid.mu.Lock()
		if raid.State == RaidStateInactive && !raid.nextOccurrence.IsZero() && now.After(raid.nextOccurrence) {
			raid.mu.Unlock()
			raid.Start(w)
		} else {
			raid.mu.Unlock()
		}
	}
}

// StartRaid starts a named raid immediately by force (uses StartRaidWithWorld).
func (rs *Raids) StartRaid(name string) error {
	rs.mu.Lock()
	_, ok := rs.raids[strings.ToLower(name)]
	rs.mu.Unlock()
	if !ok {
		return fmt.Errorf("raid not found: %s", name)
	}
	return nil
}

// StartRaidWithWorld starts a named raid with the given world reference.
func (rs *Raids) StartRaidWithWorld(name string, w *World) error {
	rs.mu.Lock()
	r, ok := rs.raids[strings.ToLower(name)]
	rs.mu.Unlock()
	if !ok {
		return fmt.Errorf("raid not found: %s", name)
	}
	r.Start(w)
	return nil
}

// MonsterType looks up a monster type by name (case-insensitive) from the world registry.
func (w *World) MonsterType(name string) *creatures.MonsterType {
	if w == nil || w.TypeRegistry == nil {
		return nil
	}
	return w.TypeRegistry.Monsters[strings.ToLower(name)]
}
