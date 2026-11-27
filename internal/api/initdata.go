package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// TGUser is the "user" field of Telegram Mini App init data.
type TGUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

var (
	errNoHash  = errors.New("init data: missing hash")
	errBadHash = errors.New("init data: invalid signature")
	errExpired = errors.New("init data: expired")
	errNoUser  = errors.New("init data: missing user")
)

// ValidateInitData checks the signature Telegram puts on Mini App launch data, as described in
// https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app
func ValidateInitData(raw, botToken string, maxAge time.Duration, now time.Time) (TGUser, error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return TGUser{}, err
	}
	hash := values.Get("hash")
	if hash == "" {
		return TGUser{}, errNoHash
	}

	pairs := make([]string, 0, len(values))
	for k := range values {
		if k != "hash" {
			pairs = append(pairs, k+"="+values.Get(k))
		}
	}
	sort.Strings(pairs)

	secret := hmacSHA256([]byte("WebAppData"), []byte(botToken))
	expected := hex.EncodeToString(hmacSHA256(secret, []byte(strings.Join(pairs, "\n"))))
	if !hmac.Equal([]byte(expected), []byte(hash)) {
		return TGUser{}, errBadHash
	}

	authDate, err := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	if err != nil || now.Sub(time.Unix(authDate, 0)) > maxAge {
		return TGUser{}, errExpired
	}

	var user TGUser
	if err := json.Unmarshal([]byte(values.Get("user")), &user); err != nil || user.ID == 0 {
		return TGUser{}, errNoUser
	}
	return user, nil
}

func hmacSHA256(key, msg []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(msg)
	return h.Sum(nil)
}
