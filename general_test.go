package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

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

func CallAPI(router *gin.Engine, method string, path string, body map[string]any) JSON {
	r := httptest.NewRecorder()
	req, e := http.NewRequest(method, path, JSONBytesBuffer(body))
	if DidFail(e, "call api") {
		panic("call api")
	}
	router.ServeHTTP(r, req)
	var result JSON
	json.Unmarshal(r.Body.Bytes(), &result.Value)
	return result
}
