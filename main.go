package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"

	"database/sql"

	_ "github.com/lib/pq"
)

const (
	isDebug = true
)

var (
	host     string
	port     int64
	user     string
	password string
	dbname   string
)

var (
	mainDB         *sql.DB
	agents         []NewsAgent
	updatingAgents bool
)

func main() {
	if isDebug {
		host = "localhost"
		port = 5432
		user = "dev"
		password = "AbstractData00"
		dbname = "socialnewsserverdev"
	} else {
		host = os.Getenv("DB_HOSTNAME")
		port, _ = strconv.ParseInt(os.Getenv("DB_PORT"), 10, 64)
		user = os.Getenv("DB_USERNAME")
		password = os.Getenv("DB_PASSWORD")
		dbname = os.Getenv("DB_DATABASE")
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()
	router.LoadHTMLFiles("./templates/verify_confirmed.html", "./templates/verify_failed.html")
	router.GET("/", index)

	api := router.Group("/api")
	v1 := api.Group("/v1")
	{
		// Sign in
		v1.POST("/auth/callbacks/sign-in", APISignIn)
		v1.POST("/auth/callbacks/sign-in-with-apple", APISignInWithApple)
		v1.POST("/auth/callbacks/sign-out", APISignOut)
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
		v1.GET("/users/:uid/recommend", APIGetSimilarPosts)
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
		v1.POST("/users/:uid/details", APIUpdateUser)
		v1.POST("/users/:uid", APIVoteUser)
		v1.POST("/posts/:pid", APIVotePost)
		v1.POST("/posts/:pid/comments/:cid", APIVoteComment)

		v1.POST("/users/:uid/content/posts/read-later", APIAddUserCont(ucpReadLater))
		v1.POST("/users/:uid/content/posts/viewed", APIAddUserCont(ucpViewed))
		v1.POST("/users/:uid/content/user-follows", APIAddUserCont(ucpUserFollow))
		v1.POST("/users/:uid/content/ignored", APIAddUserCont(ucpUserIgnored))
		v1.POST("/users/:uid/content/recommendations", APIRefreshUserContRecommendations)

		v1.POST("/users/:uid/watch/:pid", APIWatchUser)

		//Flags
		v1.POST("/flags", APICreateFlag)
		v1.GET("/flags/posts", APIGetFlaggedPosts)
		v1.GET("/flags/comments", APIGetFlaggedComments)
		v1.POST("/trash/flags/:id", APIHandleFlag)

		// Agents
		v1.POST("/agents", APICreateAgent)
		v1.POST("/agents/:aid", APIUpdateAgent)
		v1.POST("/trash/agents/:aid", APIDeleteAgent)
		v1.POST("/all-agents", APIUpdateAgents)
		v1.GET("/agents/:aid", APIGetAgent)
		v1.GET("/agents", APIGetAllAgents)
		v1.POST("/update/agents/:aid", APIEditAgent)
		v1.POST("/agents/:aid/sub", APIEditSubNewsAgent)
	}

	conn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s", host, port, user, password, dbname)

	mainDB, _ = sql.Open("postgres", conn)
	e := mainDB.Ping()
	if DidFail(e, "failed connect to db") {
		fmt.Println(conn)
	}
	defer mainDB.Close()

	// DBClearAllTables(db)

	DBSetup(mainDB)

	// SendValidationKey(10, "letanyan@icloud.com", 6347)

	agents = NAReadAllNewsAgents()
	updatingAgents = false

	NARegisterUpdates()
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
