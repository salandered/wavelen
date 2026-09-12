package user

import (
	"time"

	"github.com/salandered/strvalid"
)

type ID int64

const (
	MinNicknameLen = 3
	MaxNicknameLen = 30
)

// Lowercase ASCII letters, digits, '_' and '-'; the first and last character are letters.
// Repeated separators are ok
var nicknameCfg = strvalid.Config{
	Subject: "nickname",
	MinLen:  MinNicknameLen,
	MaxLen:  MaxNicknameLen,

	Digits: true,

	Underscore:     strvalid.SepInner,
	Dash:           strvalid.SepInner,
	AllowRepeatSep: true,

	EchoValue: false,
}

type User struct {
	ID           ID
	Nickname     string // the login identifier, unique
	PasswordHash []byte // bcrypt hash
	CreatedAt    time.Time
}

// NormalizeNickname trims and lowercases s.
// TODO: consider not lowercasing here, we store the nickname as citext anyway.
func NormalizeNickname(s string) (string, error) {
	nick := strvalid.Normalize(s, strvalid.NormalizeConfig{TrimSpaces: true, Lowercase: true})
	if err := strvalid.Validate(nick, nicknameCfg); err != nil {
		return "", err
	}
	return nick, nil
}
