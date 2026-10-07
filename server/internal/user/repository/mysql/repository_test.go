package mysql

import (
	"errors"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/kanhai447/GIM/server/internal/user/repository"
	"gorm.io/gorm"
)

func TestMapError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "not found", err: gorm.ErrRecordNotFound, want: repository.ErrNotFound},
		{name: "duplicate", err: &mysqldriver.MySQLError{Number: 1062, Message: "private database detail"}, want: repository.ErrDuplicateAccount},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := mapError("test", test.err); !errors.Is(err, test.want) {
				t.Fatalf("mapError() = %v, want %v", err, test.want)
			}
		})
	}
}

func TestRecordRoundTripPreservesInternalFields(t *testing.T) {
	record := userRecord{ID: 7, Account: "gim-user", PasswordHash: "private-hash", Nickname: "GIM", Role: 2, Status: 1}
	user := record.toDomain()
	if user.ID != record.ID || user.Account != record.Account || user.PasswordHash != record.PasswordHash {
		t.Fatalf("toDomain() lost fields: %#v", user)
	}
	if converted := recordFromDomain(user); converted != record {
		t.Fatalf("recordFromDomain() = %#v, want %#v", converted, record)
	}
}
