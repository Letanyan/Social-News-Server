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
		v1.POST("/auth/verification/users/:uid", APIResendVerificationLink)
		v1.GET("/users/:uid/verification/:key", APIVerifyUserEmail)
		// Create
		v1.POST("/users", APICreateUser)
		v1.POST("/posts", APICreatePost)
		v1.POST("/posts/:pid/comments", APICreateComment)
		// Delete
		v1.POST("/trash/users/:uid", APIDeleteUser)
		v1.POST("/trash/posts/:pid", APIDeletePost)
		v1.POST("/trash/posts/:pid/comments/:cid", APIDeleteComment)

		v1.POST("/trash/users/:uid/content/posts/read-later/:pid", APIDeleteUserContPlaylist(ucpReadLater))
		v1.POST("/trash/users/:uid/content/posts/viewed/:pid", APIDeleteUserContPlaylist(ucpViewed))
		v1.POST("/trash/users/:uid/content/user-follows/:pid", APIDeleteUserContPlaylist(ucpUserFollow))
		v1.POST("/trash/users/:uid/content/ignored/:pid", APIDeleteUserContPlaylist(ucpUserIgnored))

		// Get
		v1.GET("/users/:uid", APIGetUser)
		v1.GET("/users/:uid/prefs/users", APIGetUserPrefUsers)
		v1.GET("/users/:uid/prefs/posts", APIGetUserPrefPosts)
		v1.GET("/users/:uid/prefs/comments", APIGetUserPrefComments)
		v1.GET("/users/:uid/prefs/tags", APIGetUserPrefTags)
		v1.GET("/users/:uid/content/posts", APIGetUserContPost(ucpCreated))
		v1.GET("/users/:uid/content/posts/read-later", APIGetUserContPost(ucpReadLater))
		v1.GET("/users/:uid/content/posts/viewed", APIGetUserContPost(ucpViewed))
		v1.GET("/users/:uid/content/comments", APIGetUserContComments)
		v1.GET("/users/:uid/content/user-follows", APIGetUserContUsers(ucpUserFollow))
		v1.GET("/users/:uid/content/ignored", APIGetUserContUsers(ucpUserIgnored))
		v1.GET("/users", APIGetUsers)
		v1.GET("/posts/:pid", APIGetPost)
		v1.GET("/posts", APIGetPosts)
		v1.GET("/posts/:pid/comments/:cid", APIGetComment)
		v1.GET("/posts/comments", APIGetComments)
		v1.GET("/tags/:tid", APIGetTag)
		v1.GET("/tags", APIGetTags)
		v1.POST("/tags", APIGetTagsFromIDs)
		// Vote
		v1.POST("/credits/:uid", APIPurchaseCredit)
		v1.POST("/users/:uid", APIVoteUser)
		v1.POST("/posts/:pid", APIVotePost)
		v1.POST("/posts/:pid/comments/:cid", APIVoteComment)

		v1.POST("/users/:uid/content/posts/read-later", APIAddUserCont(ucpReadLater))
		v1.POST("/users/:uid/content/posts/viewed", APIAddUserCont(ucpViewed))
		v1.POST("/users/:uid/content/user-follows", APIAddUserCont(ucpUserFollow))
		v1.POST("/users/:uid/content/ignored", APIAddUserCont(ucpUserIgnored))
		v1.POST("/users/:uid/content/recommendations", APIRefreshUserContRecommendations)

		v1.POST("/users/:uid/watch", APIWatchUser)

		//Flags
		v1.POST("/flags", APICreateFlag)
		v1.GET("/flags", APIGetFlags)
		v1.POST("/trash/flags/:id", APIHandleFlag)
	}

	apih := router.Group("/apih")
	hv1 := apih.Group("/v1")
	{
		hv1.POST("/agents", APIHCreateAgent)
		hv1.POST("/agents/:aid", APIHUpdateAgent)
		hv1.DELETE("/agents/:aid", APIHDeleteAgent)
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

	NARegisterUpdates()
}

func index(c *gin.Context) {
	c.String(http.StatusOK, "Index")
}
