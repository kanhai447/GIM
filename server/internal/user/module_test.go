package user

import (
	"testing"

	"gorm.io/gorm"
)

func TestNewAssemblesServiceWithoutGlobalState(t *testing.T) {
	module := New(&gorm.DB{})
	if module == nil || module.Service == nil {
		t.Fatal("New() did not assemble the user service")
	}
}
