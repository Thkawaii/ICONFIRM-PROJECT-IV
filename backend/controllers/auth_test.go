package controllers

import (
	"testing"

	"iconfirm/middleware"

	"github.com/golang-jwt/jwt/v5"
)

func TestLoginSuccessIssuesJWT(t *testing.T) {
	db := newTestDB(t)
	makeUser(t, db, "wh@kobelco.com", "wh07.kobelco", "นายวสันต์", "WH")

	body := `{"username":"wh@kobelco.com","password":"wh07.kobelco"}`
	c, rec := newContext("POST", body, 0, "")
	Login(c)

	mustStatus(t, rec, 200)
	resp := decodeJSON(t, rec)

	tokenStr, ok := resp["token"].(string)
	if !ok || tokenStr == "" {
		t.Fatal("expected a token in response")
	}
	if resp["name"] != "นายวสันต์" {
		t.Errorf("name = %v, want นายวสันต์", resp["name"])
	}

	parsed, err := jwt.Parse(tokenStr, func(tk *jwt.Token) (interface{}, error) {
		return middleware.JwtKey, nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("token invalid: %v", err)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	db := newTestDB(t)
	makeUser(t, db, "wh@kobelco.com", "correct-pass", "WH", "WH")

	body := `{"username":"wh@kobelco.com","password":"wrong"}`
	c, rec := newContext("POST", body, 0, "")
	Login(c)

	mustStatus(t, rec, 401)
}

func TestLoginUnknownUser(t *testing.T) {
	newTestDB(t)

	body := `{"username":"ghost@kobelco.com","password":"x"}`
	c, rec := newContext("POST", body, 0, "")
	Login(c)

	mustStatus(t, rec, 401)
}

func TestLoginSharedUsernameResolvesByPassword(t *testing.T) {
	db := newTestDB(t)
	makeUser(t, db, "wh@kobelco.com", "pass-A", "พนักงาน A", "WH")
	makeUser(t, db, "wh@kobelco.com", "pass-B", "พนักงาน B", "WH")

	body := `{"username":"wh@kobelco.com","password":"pass-B"}`
	c, rec := newContext("POST", body, 0, "")
	Login(c)

	mustStatus(t, rec, 200)
	resp := decodeJSON(t, rec)
	if resp["name"] != "พนักงาน B" {
		t.Errorf("resolved name = %v, want พนักงาน B", resp["name"])
	}
}

func TestLoginInactiveUser(t *testing.T) {
	db := newTestDB(t)
	u := makeUser(t, db, "old@kobelco.com", "pass", "เก่า", "WH")
	db.Model(&u).Update("status", "Inactive")

	body := `{"username":"old@kobelco.com","password":"pass"}`
	c, rec := newContext("POST", body, 0, "")
	Login(c)

	mustStatus(t, rec, 403)
}
