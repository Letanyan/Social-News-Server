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
	agents []NewsAgent
)

func main() {
	router := gin.Default()
	router.GET("/", index)

	api := router.Group("/api")
	v1 := api.Group("/v1")
	{
		// Sign in
		v1.POST("/auth/callbacks/sign-in", APISignIn)
		v1.POST("/auth/callbacks/sign-in-with-apple", APISignInWithApple)
		// Create
		v1.POST("/users", APICreateUser)
		v1.POST("/posts", APICreatePost)
		v1.POST("/posts/:pid/comments", APICreateComment)
		// Delete
		v1.DELETE("/users/:uid", APIDeleteUser)
		v1.DELETE("/posts/:pid", APIDeletePost)
		v1.DELETE("/posts/:pid/comments/:cid", APIDeleteComment)
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
		v1.POST("/credits/:uid", APIPurchaseCredit)
		v1.POST("/users/:uid", APIVoteUser)
		v1.POST("/posts/:pid", APIVotePost)
		v1.POST("/posts/:pid/comments/:cid", APIVoteComment)

		v1.POST("/users/:uid/watch", APIWatchUser)
		v1.GET("/users/:uid/blacklist/:tid", APIBlacklistUser)

		//Flags
		v1.POST("/flags", APICreateFlag)
		v1.GET("/flags", APIGetFlags)
		v1.DELETE("/flags/:id", APIHandleFlag)
	}

	apih := router.Group("/apih")
	hv1 := apih.Group("/v1")
	{
		hv1.POST("/agents", APIHCreateAgent)
		hv1.POST("/agents/:aid", APIHUpdateAgent)
		hv1.POST("/all-agents", APIHUpdateAgents)
		hv1.GET("/agents/:aid", APIHGetAgent)
		hv1.GET("/agents", APIHGetAllAgents)
	}

	conn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s", host, port, user, password, dbname)

	mainDB, _ = sql.Open("postgres", conn)
	defer mainDB.Close()

	// DBClearAllTables(db)

	DBSetup(mainDB)

	agents = NAReadAllNewsAgents()

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
