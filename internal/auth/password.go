package auth

import (
	"golang.org/x/crypto/bcrypt"
)

// bcryptCost is the cost factor for bcrypt hashing
const bcryptCost = 12

// Hash generates a bcrypt hash of the provided password
func Hash(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// Verify compares a bcrypt hashed password with a plain text password
func Verify(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}
