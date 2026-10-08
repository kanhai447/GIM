package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUserAndPublicInfoNeverMarshalPasswordHash(t *testing.T) {
	user := User{
		ID:           42,
		Account:      "gim-user",
		PasswordHash: "sensitive-password-hash",
		OpenID:       "private-provider-identity",
		Nickname:     "GIM User",
		Role:         RoleMember,
		Status:       StatusActive,
	}

	for name, value := range map[string]any{"user": user, "public info": user.PublicInfo()} {
		payload, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("json.Marshal(%s) error = %v", name, err)
		}
		if strings.Contains(string(payload), user.PasswordHash) || strings.Contains(string(payload), user.OpenID) || strings.Contains(string(payload), "PasswordHash") {
			t.Fatalf("json.Marshal(%s) exposed private identity data", name)
		}
	}
}
