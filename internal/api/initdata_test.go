package api

import (
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

const testToken = "123456:TEST-TOKEN"

// sign builds init data the same way Telegram does.
func sign(values url.Values, token string) string {
	var pairs []string
	for k := range values {
		pairs = append(pairs, k+"="+values.Get(k))
	}
	sort.Strings(pairs)
	secret := hmacSHA256([]byte("WebAppData"), []byte(token))
	values.Set("hash", hex.EncodeToString(hmacSHA256(secret, []byte(strings.Join(pairs, "\n")))))
	return values.Encode()
}

func initData(authDate time.Time) url.Values {
	return url.Values{
		"auth_date": {strconv.FormatInt(authDate.Unix(), 10)},
		"query_id":  {"AAH"},
		"user":      {`{"id":42,"first_name":"Ali","username":"ali"}`},
	}
}

func TestValidateInitData(t *testing.T) {
	now := time.Now()
	raw := sign(initData(now.Add(-time.Minute)), testToken)

	user, err := ValidateInitData(raw, testToken, time.Hour, now)
	if err != nil || user.ID != 42 || user.FirstName != "Ali" {
		t.Fatalf("valid data rejected: %v %+v", err, user)
	}
	if _, err := ValidateInitData(raw, "other:token", time.Hour, now); err != errBadHash {
		t.Errorf("wrong token: got %v", err)
	}
	tampered := strings.Replace(raw, "42", "43", 1)
	if _, err := ValidateInitData(tampered, testToken, time.Hour, now); err != errBadHash {
		t.Errorf("tampered user: got %v", err)
	}
	old := sign(initData(now.Add(-2*time.Hour)), testToken)
	if _, err := ValidateInitData(old, testToken, time.Hour, now); err != errExpired {
		t.Errorf("expired: got %v", err)
	}
	if _, err := ValidateInitData("auth_date=1", testToken, time.Hour, now); err != errNoHash {
		t.Errorf("no hash: got %v", err)
	}
}
