package mysql

import (
	"errors"
	"fmt"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"
)

func TestDeadlockClassification(t *testing.T) {
	deadlock := fmt.Errorf("transaction failed: %w", &mysqldriver.MySQLError{Number: 1213})
	if !isDeadlock(deadlock) {
		t.Fatal("wrapped MySQL 1213 must be retryable")
	}
	if isDeadlock(&mysqldriver.MySQLError{Number: 1205}) || isDeadlock(errors.New("deadlock")) {
		t.Fatal("only MySQL 1213 may use the bounded retry")
	}
}
