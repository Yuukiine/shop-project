package models

type App struct {
	ID     int    `json:"id" db:"app_id"`
	Name   string `json:"name" db:"name"`
	Secret string `json:"secret" db:"secret"`
}
