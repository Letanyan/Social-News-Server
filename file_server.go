package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func serveHTML(c *gin.Context, file string, obj any) {
	c.Header("Access-Control-Allow-Origin", "*")         // Required for CORS support to work
	c.Header("Access-Control-Allow-Credentials", "true") // Required for cookies, authorization headers with HTTPS
	c.Header("Access-Control-Allow-Headers", "Origin,Content-Type,X-Amz-Date,Authorization,X-Api-Key,X-Amz-Security-Token,locale")
	c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE")
	c.HTML(http.StatusOK, file, obj)
}

func ServeFiles(router *gin.Engine) {
	router.LoadHTMLFiles(
		"./templates/verify.html",
		"./templates/verify_confirmed.html",
		"./templates/verify_failed.html",
		"./templates/reset_password.html",
		"./templates/reset_password_form.html",
		"./templates/reset_password_confirmed.html",
		"./templates/reset_password_failed.html",
		"./templates/privacy.html",
	)
	router.GET("/", serveIndex)
	router.GET("/privacy", servePrivacy)
}

func servePrivacy(c *gin.Context) {
	serveHTML(c, "privacy.html", gin.H{})
}

func serveIndex(c *gin.Context) {
	c.String(http.StatusOK, "Index")
}
