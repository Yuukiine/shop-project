package models

type User struct {
	ID       int    `json:"id" db:"id"`
	Email    string `json:"email" db:"email"`
	PassHash []byte `json:"pass_hash" db:"pass_hash"`
}
