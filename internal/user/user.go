package user

import (
	"time"

	"github.com/salandered/strvalid/strvalid"
)

type ID int64

const (
	MinNameLen     = 1
	MaxNameLen     = 60
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

var nameCfg = strvalid.UnicodeConfig{
	Subject:  "name",
	MinRunes: MinNameLen,
	MaxRunes: MaxNameLen,
}

type User struct {
	ID           ID
	Nickname     string // the login identifier, unique
	Name         string // free-form, for display
	PasswordHash []byte // bcrypt hash
	CreatedAt    time.Time
}

// NormalizeNickname trims and lowercases s.
// TODO: consider not lowercasing here, we store the nickname as citext anyway.
func NormalizeNickname(s string) (string, error) {
	nick := strvalid.Normalize(s, true, true)
	if err := strvalid.Validate(nick, nicknameCfg); err != nil {
		return "", err
	}
	return nick, nil
}

// NormalizeName trims s.
func NormalizeName(s string) (string, error) {
	name := strvalid.Normalize(s, true, false)
	if err := strvalid.ValidateUnicode(name, nameCfg); err != nil {
		return "", err
	}
	return name, nil
}
