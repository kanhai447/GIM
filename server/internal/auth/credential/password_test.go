package credential

import "testing"

func TestPasswordHashDiffersAndVerifies(t *testing.T) {
	passwords := NewPasswords(bcryptTestCost)
	plaintext := "correct-horse-battery-staple"
	hash, err := passwords.Hash(plaintext)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	if hash == plaintext {
		t.Fatal("Hash() returned plaintext")
	}
	if err := passwords.Compare(hash, plaintext); err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if err := passwords.Compare(hash, "wrong-password"); err == nil {
		t.Fatal("Compare() accepted a wrong password")
	}
}

const bcryptTestCost = 4
