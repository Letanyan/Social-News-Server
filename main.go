package main

import (
	"fmt"
	"log"
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
	mainDB       *sql.DB
	agents       []NewsAgent
	agentsMutex  KeyedMutex
	englishWords map[string]bool
	serverAddr   string

	agentsOnboarding map[string]int64
	tagsOnboarding   map[string]int64
)

func main() {
	if isDebug {
		host = "localhost"
		port = 5432
		user = "dev"
		password = "AbstractData00"
		dbname = "socialnewsserverdev"
		serverAddr = "http://localhost:8080"
	} else {
		host = os.Getenv("DB_HOSTNAME")
		port, _ = strconv.ParseInt(os.Getenv("DB_PORT"), 10, 64)
		user = os.Getenv("DB_USERNAME")
		password = os.Getenv("DB_PASSWORD")
		dbname = os.Getenv("DB_DATABASE")
		gin.SetMode(gin.ReleaseMode)
		serverAddr = "https://new-source-server-mhvly.ondigitalocean.app"
	}

	conn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s", host, port, user, password, dbname)

	mainDB, _ = sql.Open("postgres", conn)
	e := mainDB.Ping()
	if DidFail(e, "failed connect to db") {
		fmt.Println(conn)
	}
	defer mainDB.Close()

	DBSetup(mainDB)

	agents = NAReadAllNewsAgents(mainDB)
	agentsOnboarding = NAReadAllAgentsOnboarding()
	tagsOnboarding = NAReadAllTagsOnboarding(mainDB)
	agentsMutex = KeyedMutex{}
	NARegisterHourlyUpdates(mainDB)
	NARegisterWeeklyCleanUp(mainDB)

	router := gin.Default()
	ServeFiles(router)
	ServeAPI(router)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if err := router.Run(":" + port); err != nil {
		log.Panicf("error: %s", err.Error())
	}
}
