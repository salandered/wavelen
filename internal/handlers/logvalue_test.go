package handlers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const secretInTest = "correct horse battery"

func TestCreateUserReqLogValueRedactsPassword(t *testing.T) {
	req := CreateUserReq{Nickname: "olya", Name: "Olya", Password: secretInTest}

	logged := req.LogValue().String()

	require.Contains(t, logged, "olya")
	require.NotContains(t, logged, secretInTest)
}

func TestCreateTokenReqLogValueRedactsPassword(t *testing.T) {
	req := CreateTokenReq{Nickname: "olya", Password: secretInTest}

	logged := req.LogValue().String()

	require.Contains(t, logged, "olya")
	require.NotContains(t, logged, secretInTest)
}

func TestCreateTokenRespLogValueRedactsToken(t *testing.T) {
	expiry := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	resp := CreateTokenResp{Token: "X3ASTT2CDAN66BACKSCI4SU7SI", Expiry: expiry}

	logged := resp.LogValue().String()

	require.Contains(t, logged, "2026-09-12")
	require.NotContains(t, logged, "X3ASTT2CDAN66BACKSCI4SU7SI")
}
