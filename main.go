package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"database/sql"

	"github.com/lib/pq"
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
		v1.POST("/credits/:uid", APIPurchaseCredit)
		v1.POST("/users/:uid", APIVoteUser)
		v1.POST("/posts/:pid", APIVotePost)
		v1.POST("/posts/:pid/comments/:cid", APIVoteComment)
	}

	conn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s", host, port, user, password, dbname)

	mainDB, _ = sql.Open("postgres", conn)
	defer mainDB.Close()

	// DBClearAllTables(db)

	DBSetup(mainDB)

	query := fmt.Sprintf(`
    WITH tv AS (
        SELECT t.name, ratio(up.upvotes, up.downvotes) AS val
        FROM user3pref up JOIN tags t ON t.id = up.pid
        WHERE up.kind = 4
    )
    SELECT id, upvotes, tags, 
        SUM(cooldown(p.upvotes - p.downvotes, p.updatedAt, now() at time zone ('utc'), 31536000) * tv.val),
        p.upvotes - p.downvotes AS x, tv.val AS y, tv.name
    FROM posts2022 p JOIN tv ON ARRAY[tv.name] <@ p.tags
    GROUP BY id, upvotes, tags, x, y, tv.name
    `)
	rows, e := mainDB.Query(query)
	if DidFail(e) {
		return
	}
	for rows.Next() {
		var p PostResult
		var s float64
		var x float64
		var y float64
		var name string
		e = rows.Scan(&p.ID, &p.Content, pq.Array(&p.Tags), &s, &x, &y, &name)
		if DidFail(e) {
			return
		}

		fmt.Println(p.ID, p.Content, p.Tags, s, x, y, name)
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
