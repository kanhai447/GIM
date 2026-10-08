// Package credential owns password hashing and comparison for Auth.
package credential

import "golang.org/x/crypto/bcrypt"

const DefaultBcryptCost = 12

type Passwords struct{ cost int }

func NewPasswords(cost int) Passwords {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		cost = DefaultBcryptCost
	}
	return Passwords{cost: cost}
}

func (passwords Passwords) Hash(plaintext string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), passwords.cost)
	return string(hash), err
}

func (passwords Passwords) Compare(hash, plaintext string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext))
}
