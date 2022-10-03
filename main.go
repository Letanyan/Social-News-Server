package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"database/sql"

	_ "github.com/lib/pq"
)

const (
	isDebug  = true
	host     = "localhost"
	port     = 5432
	user     = "dev"
	password = "AbstractData00"
	dbname   = "socialnewsserverdev"
)

func main() {
	router := gin.Default()
	router.GET("/", index)
	router.GET("/albums", getAlbums)

	conn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s", host, port, user, password, dbname)

	db, _ := sql.Open("postgres", conn)
	defer db.Close()

	// DBDeleteAllPosts(db)
	// DBDeleteAllUsers(db)
	// DBDeleteTable(db, "tags")

	DBSetup(db)

	// createNewUser := func() {
	// 	name := "Adam"
	// 	email := "adam@social.com"
	// 	pass := "hfusi"
	// 	if DBIsValidEmail(db, email) {
	// 		DBCreateUser(db, name, email, pass)
	// 	} else {
	// 		log.Println("Failed to create new user email is taken")
	// 	}
	// }
	// createNewUser()

	// DBCreatePost(db, 1, "This is a post", []string{})
	// DBCreatePost(db, 1, "This is another post", []string{})
	// DBCreatePost(db, 1, "What do you say", []string{})

	// DBCreateComment(db, 1, "This is a comment", 1, 0)
	// DBCreateComment(db, 1, "This is a reply", 1, 1)

	// DBCreatePost(db, 1, "Where are we", []string{"lost", "going places"})
	// DBCreatePost(db, 1, "Returning trip", []string{})

	DBVotePost(db, 1, 2, -2)
	DBVotePost(db, 1, 4, 2)
	DBVotePost(db, 1, 4, 2)
	DBVoteComment(db, 1, 1, 1, 5)

	posts := DBGetPosts(db, 1, []string{}, soCreatedAt, 10, 0, "2022-01-01 00:00:00.00", "2023-01-01 00:00:00.00")
	for _, p := range posts {
		log.Println(p)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if err := router.Run(":" + port); err != nil {
		log.Panicf("error: %s", err.Error())
	}
}

func index(c *gin.Context) {
	c.String(http.StatusOK, "Index")
}

// getAlbums responds with the list of all albums as JSON.
func getAlbums(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, "[[Albums]]")
}
