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

var (
	mainDB *sql.DB
)

func main() {
	router := gin.Default()
	router.GET("/", index)

	api := router.Group("/api")
	v1 := api.Group("/v1")
	{
		// Create
		v1.POST("/users", APICreateUser)
		v1.POST("/posts", APICreatePost)
		// Delete
		v1.DELETE("/users/:uid", APIDeleteUser)
		v1.DELETE("/posts/:pid", APICreateUser)
		// Get
		v1.GET("/users/:uid", APIGetUser)
		v1.GET("/users/:uid/prefs/users", APIGetUserPrefUsers)
		v1.GET("/users/:uid/prefs/posts", APIGetUserPrefPosts(false))
		v1.GET("/users/:uid/prefs/comments", APIGetUserPrefPosts(true))
		v1.GET("/users/:uid/prefs/tags", APIGetUserPrefTags)
		v1.GET("/users/:uid/content/posts", APIGetUserContent(true))
		v1.GET("/users/:uid/content/comments", APIGetUserContent(false))
		v1.GET("/users", APIGetUsers)
		v1.GET("/posts/:pid", APIGetPost)
		v1.GET("/posts", APIGetPosts)
		v1.GET("/posts/:pid/comments/:cid", APIGetComment)
		v1.GET("/posts/:pid/comments", APIGetComments)
		v1.GET("/tags/:tid", APIGetTag)
		v1.GET("/tags", APIGetTags)
		// Vote
		v1.POST("/users/:uid", APIVoteUser)
		v1.POST("/posts/:pid", APIVotePost)
		v1.POST("/posts/:pid/comments/:cid", APIVoteComment)
	}

	conn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s", host, port, user, password, dbname)

	mainDB, _ = sql.Open("postgres", conn)
	defer mainDB.Close()

	// DBClearAllTables(db)

	DBSetup(mainDB)

	// loc1 := []string{"Africa", "South Africa", "Gauteng", "Centurion"}
	// loc2 := []string{"Asia", "Japan", "Tokyo", "Chiyoda"}
	// loc3 := []string{}

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

	// DBCreatePost(db, 1, "This is a post", []string{}, loc1)
	// DBCreatePost(db, 1, "This is another post", []string{}, loc3)
	// DBCreatePost(db, 1, "What do you say", []string{}, loc1)

	// DBCreateComment(db, 1, "This is a comment", 221003162748045645, 0)
	// DBCreateComment(db, 1, "This is a reply", 221003162748045645, 1)

	// DBCreatePost(db, 1, "Where are we", []string{"lost", "going places"}, loc2)
	// DBCreatePost(db, 1, "Returning trip", []string{}, loc1)

	// DBVotePost(db, 1, 221003194241748012, -2, loc1)
	// DBVotePost(db, 1, 221003194241787611, 2, loc2)
	// DBVotePost(db, 1, 221003194241787611, 2, loc3)
	// DBVoteComment(db, 1, 221003194241748012, 1, 5)

	// posts := DBGetPosts(db, 1, []string{}, []string{"Africa"}, soCreatedAt, 10, 0, "2022-01-01 00:00:00.00", "2023-01-01 00:00:00.00")
	// for _, p := range posts {
	// 	log.Println(p)
	// }

	// prefs := DBGetUserPref(db, 1, soScore, upPost, 0, 0, 10, 0)
	// for _, p := range prefs {
	// 	log.Printf("%v\n", p)
	// }

	// tags := DBGetTags(db, []string{}, []string{}, soUpvotes, 10, 0)
	// for _, t := range tags {
	// 	log.Println(t)
	// }

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
