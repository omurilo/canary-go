package mounts

import (
	"encoding/xml"
	"os"
)

type Mount struct {
	ID       uint16 `xml:"id,attr"`
	ClientID uint16 `xml:"clientid,attr"`
	Name     string `xml:"name,attr"`
	Speed    int32  `xml:"speed,attr"`
	Premium  string `xml:"premium,attr"`
	Type     string `xml:"type,attr"`
}

type mountsXML struct {
	XMLName xml.Name `xml:"mounts"`
	Mounts  []Mount  `xml:"mount"`
}

var (
	byID       = make(map[uint16]Mount)
	byClientID = make(map[uint16]Mount)
	allMounts  []Mount
)

// Load reads and parses mounts.xml from the specified file path.
func Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var mx mountsXML
	if err := xml.Unmarshal(data, &mx); err != nil {
		return err
	}
	byID = make(map[uint16]Mount)
	byClientID = make(map[uint16]Mount)
	allMounts = mx.Mounts
	for _, m := range mx.Mounts {
		byID[m.ID] = m
		byClientID[m.ClientID] = m
	}
	return nil
}

// GetByID returns the mount for a given mount ID (1-based ID from mounts.xml).
func GetByID(id uint16) (Mount, bool) {
	m, ok := byID[id]
	return m, ok
}

// GetByClientID returns the mount for a given client outfit ID (e.g. 368).
func GetByClientID(clientId uint16) (Mount, bool) {
	m, ok := byClientID[clientId]
	return m, ok
}

// All returns a slice of all registered mounts.
func All() []Mount {
	return allMounts
}
