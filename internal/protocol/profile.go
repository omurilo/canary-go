package protocol

import (
	"fmt"
	"strings"

	"github.com/opentibiabr/canary-go/internal/transport"
)

// ProtocolVersion identifies a supported client protocol version family.
type ProtocolVersion uint16

const (
	VersionCurrent  ProtocolVersion = 1525 // Tibia 13.x (modern)
	VersionTibia1100 ProtocolVersion = 1100 // Tibia 11.00
	VersionCipsoft860 ProtocolVersion = 860 // Tibia 8.60
)

// Profile bundles the wire-format and behavioral settings for a protocol
// version, mirroring the C++ ProtocolProfile concept.
type Profile struct {
	Version    ProtocolVersion
	Label      string // human-readable label, e.g. "13.x", "11.00", "8.60"
	ClientStr  string // reported in status XML, e.g. "13.15", "11.00", "8.60"

	// Transport settings.
	Transport     transport.ProfileID
	CryptoMethod  transport.CryptoMethod

	// Login layout.
	HasContentRevision bool // client version string includes a u16 content revision
	HasPreviewState    bool // first packet has a preview-state byte
	PasswordLogin      bool // true = legacy password auth instead of session key

	// Game features.
	HasChallengeResponse bool // challenge/response in game login handshake
	HasOtcV8Probe        bool // OTCv8 probe byte in game login
	BlockMemorial        bool // block memorial window opcodes (not in 860/1100)
	BlockWeaponProficiency bool // block weapon-detail packets (not in 860/1100)
	LegacyTalkClasses    bool // map speak/message classes to 8.6 values
	CapExperience        bool // cap experience at int32 max (8.6)
	SuppressPreLoginPackets bool // suppress some packets before login
	HasProtocolDiagnostics  bool // send protocol version info on connect
}

// currentProfile is the Tibia 13.x (protocol 1525) profile.
var currentProfile = &Profile{
	Version:    VersionCurrent,
	Label:      "13.x",
	ClientStr:  "13.15",
	Transport:     transport.ProfileCurrentModern,
	CryptoMethod:  transport.CryptoSequence,
	HasContentRevision: true,
	HasPreviewState:    true,
	PasswordLogin:      false,
	HasChallengeResponse: true,
	HasOtcV8Probe:        true,
	BlockMemorial:        false,
	BlockWeaponProficiency: false,
	LegacyTalkClasses:    false,
	CapExperience:        false,
	SuppressPreLoginPackets: false,
	HasProtocolDiagnostics:  true,
}

// tibia1100Profile is the legacy Tibia 11.00 profile.
var tibia1100Profile = &Profile{
	Version:    VersionTibia1100,
	Label:      "11.00",
	ClientStr:  "11.00",
	Transport:     transport.ProfileLegacyLogin,
	CryptoMethod:  transport.CryptoAdler32,
	HasContentRevision: true,
	HasPreviewState:    false,
	PasswordLogin:      true,
	HasChallengeResponse: true,
	HasOtcV8Probe:        false,
	BlockMemorial:        true,
	BlockWeaponProficiency: true,
	LegacyTalkClasses:    false,
	CapExperience:        false,
	SuppressPreLoginPackets: false,
	HasProtocolDiagnostics:  true,
}

// cipsoft860Profile is the legacy Tibia 8.60 profile.
var cipsoft860Profile = &Profile{
	Version:    VersionCipsoft860,
	Label:      "8.60",
	ClientStr:  "8.60",
	Transport:     transport.ProfileLegacyClassic,
	CryptoMethod:  transport.CryptoAdler32,
	HasContentRevision: false,
	HasPreviewState:    false,
	PasswordLogin:      true,
	HasChallengeResponse: false,
	HasOtcV8Probe:        false,
	BlockMemorial:        true,
	BlockWeaponProficiency: true,
	LegacyTalkClasses:    true,
	CapExperience:        true,
	SuppressPreLoginPackets: true,
	HasProtocolDiagnostics:  false,
}

// ProfileForPort returns the protocol profile for the given game port, or nil
// if the port does not match any known game protocol version.
func ProfileForPort(port int, cfgProfile *Profile) *Profile {
	if port == 0 {
		return nil
	}
	// Use the explicitly set profile if provided.
	if cfgProfile != nil {
		return cfgProfile
	}
	return currentProfile
}

// ProfileForVersion returns the protocol profile for the given client version
// number, or nil if it is not a recognised legacy version (callers should fall
// back to currentProfile).
func ProfileForVersion(version uint32, allowOld bool) *Profile {
	switch ProtocolVersion(version) {
	case VersionTibia1100:
		if allowOld {
			return tibia1100Profile
		}
		return nil
	case VersionCipsoft860:
		if allowOld {
			return cipsoft860Profile
		}
		return nil
	default:
		return currentProfile
	}
}

// ProfileName returns the display name for a profile, e.g. "13.15 (1525)".
func ProfileName(p *Profile) string {
	if p == nil {
		return "unknown"
	}
	return fmt.Sprintf("%s (%d)", p.ClientStr, p.Version)
}

// VersionStringFromClient parses the client version string (e.g. "13.15.12345")
// and returns just the version component. Helper for legacy content revision.
func VersionStringFromClient(s string) string {
	if idx := strings.IndexByte(s, '.'); idx >= 0 {
		if idx2 := strings.IndexByte(s[idx+1:], '.'); idx2 >= 0 {
			return s[:idx+1+idx2]
		}
	}
	return s
}
