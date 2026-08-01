package protocol

import (
	"context"
	"encoding/json"
	"time"

	"github.com/omurilo/canary-go/internal/db"
	"github.com/omurilo/canary-go/internal/network"
)

// The HTTP/JSON login the modern client uses.
//
// The official 13.x client does not use the binary login on 7171 at all — it
// POSTs {"type":"login", ...} and expects a session plus the world and
// character lists back. Upstream Canary does not serve this either; the
// deployment runs opentibiabr/login-server alongside it (see
// deploy/docker-compose.yml, port 8088), and MyAAC serves the same contract in
// a PHP stack. There is therefore no C++ function to port this from.
//
// Without it the request fell through to a 400, and the client treated that as
// a successful login with nothing in it: a wrong password reached the character
// list screen just the same, because nothing had authenticated at all.
//
// The field names below are the login-server/MyAAC contract. They are
// reconstructed from that contract rather than from source in this repository,
// so a field the client silently ignores may be wrong without anything failing
// loudly — the ones that matter are session.sessionkey, playdata.worlds and
// playdata.characters.

type loginRequest struct {
	Type         string `json:"type"`
	Email        string `json:"email"`
	AccountName  string `json:"accountname"`
	Password     string `json:"password"`
	Token        string `json:"token"`
	StayLoggedIn bool   `json:"stayloggedin"`
}

type loginSession struct {
	SessionKey                    string `json:"sessionkey"`
	LastLoginTime                 int64  `json:"lastlogintime"`
	IsPremium                     bool   `json:"ispremium"`
	PremiumUntil                  int64  `json:"premiumuntil"`
	Status                        string `json:"status"`
	ReturnerNotification          bool   `json:"returnernotification"`
	ShowRewardNews                bool   `json:"showrewardnews"`
	IsReturner                    bool   `json:"isreturner"`
	FpsTracking                   bool   `json:"fpstracking"`
	OptionTracking                bool   `json:"optiontracking"`
	TournamentTicketPurchaseState int    `json:"tournamentticketpurchasestate"`
	EmailCodeRequest              bool   `json:"emailcoderequest"`
}

type loginWorld struct {
	ID                         int    `json:"id"`
	Name                       string `json:"name"`
	ExternalAddress            string `json:"externaladdress"`
	ExternalPort               int    `json:"externalport"`
	ExternalAddressProtected   string `json:"externaladdressprotected"`
	ExternalPortProtected      int    `json:"externalportprotected"`
	ExternalAddressUnprotected string `json:"externaladdressunprotected"`
	ExternalPortUnprotected    int    `json:"externalportunprotected"`
	PreviewState               int    `json:"previewstate"`
	Location                   string `json:"location"`
	AntiCheatProtection        bool   `json:"anticheatprotection"`
	PvpType                    int    `json:"pvptype"`
	IsTournamentWorld          bool   `json:"istournamentworld"`
	RestrictedStore            bool   `json:"restrictedstore"`
	CurrentTournamentPhase     int    `json:"currenttournamentphase"`
}

type loginCharacter struct {
	WorldID                          int    `json:"worldid"`
	Name                             string `json:"name"`
	Level                            uint32 `json:"level"`
	Vocation                         string `json:"vocation"`
	OutfitID                         uint16 `json:"outfitid"`
	HeadColor                        uint16 `json:"headcolor"`
	TorsoColor                       uint16 `json:"torsocolor"`
	LegsColor                        uint16 `json:"legscolor"`
	DetailColor                      uint16 `json:"detailcolor"`
	AddonsFlags                      uint16 `json:"addonsflags"`
	IsMale                           bool   `json:"ismale"`
	Tutorial                         bool   `json:"tutorial"`
	IsHidden                         bool   `json:"ishidden"`
	IsMainCharacter                  bool   `json:"ismaincharacter"`
	DailyRewardState                 int    `json:"dailyrewardstate"`
	RemainingDailyTournamentPlayTime int    `json:"remainingdailytournamentplaytime"`
}

type loginPlayData struct {
	Worlds     []loginWorld     `json:"worlds"`
	Characters []loginCharacter `json:"characters"`
}

type loginResponse struct {
	Session  loginSession  `json:"session"`
	PlayData loginPlayData `json:"playdata"`
}

// vocationNames indexes the client-facing name by the players.vocation column.
// Promotions map back onto their base name, which is what the character list
// shows.
var vocationNames = map[uint16]string{
	0: "None", 1: "Sorcerer", 2: "Druid", 3: "Paladin", 4: "Knight",
	5: "Master Sorcerer", 6: "Elder Druid", 7: "Royal Paladin", 8: "Elite Knight",
	9: "Monk", 10: "Exalted Monk",
}

// handleJSONLogin authenticates and answers with the session and play data.
func (p *LoginProtocol) handleJSONLogin(c *network.Connection, req loginRequest) {
	identifier := req.Email
	if identifier == "" {
		identifier = req.AccountName
	}
	if identifier == "" || req.Password == "" {
		c.Logger().Info("login: json login rejected", "reason", "empty credentials")
		jsonLoginError(c, "Account name or password is not correct.")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	acc, err := p.deps.DB.LoadAccount(ctx, identifier)
	// Same message for an unknown account and a bad password, so the response
	// cannot be used to enumerate accounts.
	if err != nil || !db.VerifyPassword(req.Password, acc.Password) {
		c.Logger().Info("login: json login rejected", "account", identifier,
			"reason", "account or password incorrect")
		jsonLoginError(c, "Account name or password is not correct.")
		return
	}

	chars, err := p.deps.DB.ListCharacters(ctx, acc.ID)
	if err != nil {
		c.Logger().Warn("login: listing characters failed", "account", identifier, "err", err)
		jsonLoginError(c, "Unable to load your characters.")
		return
	}

	cfg := p.deps.Cfg
	world := loginWorld{
		Name:                       cfg.ServerName,
		ExternalAddress:            cfg.IP,
		ExternalPort:               cfg.GamePort,
		ExternalAddressProtected:   cfg.IP,
		ExternalPortProtected:      cfg.GamePort,
		ExternalAddressUnprotected: cfg.IP,
		ExternalPortUnprotected:    cfg.GamePort,
	}

	out := make([]loginCharacter, 0, len(chars))
	for _, ch := range chars {
		vocation, ok := vocationNames[ch.Vocation]
		if !ok {
			vocation = "None"
		}
		out = append(out, loginCharacter{
			Name:     ch.Name,
			Level:    ch.Level,
			Vocation: vocation,
			// PLAYERSEX_FEMALE is 0 and PLAYERSEX_MALE is 1.
			IsMale:      ch.Sex == 1,
			OutfitID:    ch.LookType,
			HeadColor:   ch.LookHead,
			TorsoColor:  ch.LookBody,
			LegsColor:   ch.LookLegs,
			DetailColor: ch.LookFeet,
			AddonsFlags: ch.LookAddons,
		})
	}

	premium := acc.PremDays > 0
	var premiumUntil int64
	if premium {
		premiumUntil = time.Now().Add(time.Duration(acc.PremDays) * 24 * time.Hour).Unix()
	}

	resp, err := json.Marshal(loginResponse{
		Session: loginSession{
			// The game protocol authenticates with this verbatim, and splits it on
			// the newline (see splitSessionKey).
			SessionKey:   identifier + "\n" + req.Password,
			Status:       "active",
			IsPremium:    premium,
			PremiumUntil: premiumUntil,
		},
		PlayData: loginPlayData{
			Worlds:     []loginWorld{world},
			Characters: out,
		},
	})
	if err != nil {
		c.Logger().Error("login: encoding the json login response failed", "err", err)
		jsonLoginError(c, "Internal error.")
		return
	}

	c.Logger().Info("login: json login ok", "account", identifier, "accountId", acc.ID,
		"characters", len(out), "world", cfg.ServerName, "ip", cfg.IP, "port", cfg.GamePort)
	sendJSON(c, resp)
}

// jsonLoginError uses the client's own error shape: errorCode is a number here,
// unlike the string the status endpoints return.
func jsonLoginError(c *network.Connection, msg string) {
	resp, _ := json.Marshal(map[string]any{"errorCode": 3, "errorMessage": msg})
	sendJSON(c, resp)
}
