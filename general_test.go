package main

import (
	"database/sql"
	"fmt"

	"github.com/gin-gonic/gin"
)

func getTestDatabase() *sql.DB {
	const (
		host     = "localhost"
		port     = 5432
		user     = "dev"
		password = "AbstractData00"
		dbname   = "spcialnewsservertest"
	)

	conn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s", host, port, user, password, dbname)

	db, _ := sql.Open("postgres", conn)

	return db
}

func getTestRouter() *gin.Engine {
	router := gin.New()
	ServeFiles(router)
	ServeAPI(router)
	return router
}
