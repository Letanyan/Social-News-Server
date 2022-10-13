package main

import (
	"database/sql"
	"fmt"
)

func getTestDatabase() *sql.DB {
	const (
		host     = "localhost"
		port     = 5432
		user     = "dev"
		password = "AbstractData00"
		dbname   = "socialnewsservertest"
	)

	conn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s", host, port, user, password, dbname)

	db, _ := sql.Open("postgres", conn)

	return db
}
