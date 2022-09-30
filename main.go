package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()
	router.GET("/", index)
	router.GET("/albums", getAlbums)

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
