package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Timothylock/go-signin-with-apple/apple"
)

func APIReturn(c *gin.Context, success bool, payload interface{}) {
	c.Header("Access-Control-Allow-Origin", "*")         // Required for CORS support to work
	c.Header("Access-Control-Allow-Credentials", "true") // Required for cookies, authorization headers with HTTPS
	c.Header("Access-Control-Allow-Headers", "Origin,Content-Type,X-Amz-Date,Authorization,X-Api-Key,X-Amz-Security-Token,locale")
	c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE")
	if isDebug {
		if success {
			c.IndentedJSON(http.StatusOK, gin.H{"success": true, "payload": payload})
		} else {
			c.IndentedJSON(http.StatusOK, gin.H{"success": false, "reason": payload})
		}
	} else {
		if success {
			c.JSON(http.StatusOK, gin.H{"success": true, "payload": payload})
		} else {
			c.JSON(http.StatusOK, gin.H{"success": false, "reason": payload})
		}
	}
}

func APIFailed(c *gin.Context, e error, reason string) bool {
	if e == nil {
		return false
	} else {
		APIReturn(c, false, reason)
		return true
	}
}

// ------------------------------------------------------------------------
// Create
// ------------------------------------------------------------------------
func APICreateUser(c *gin.Context) {
	type Input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	var input Input

	if e := c.BindJSON(&input); DidFail(e, "get input for create user") {
		APIReturn(c, false, "invalid input values")
		return
	}

	validUsername := validateLength("username", input.Name, 3, 15)
	if len(validUsername) > 0 {
		APIReturn(c, false, validUsername)
		return
	}
	validPassword := validateLength("password", input.Password, 8, 2048)
	if len(validPassword) > 0 {
		APIReturn(c, false, validPassword)
		return
	}
	if !validEmailMatch(input.Email) {
		APIReturn(c, false, "email address appears to be invalid")
		return
	}

	emailTaken := DBContainsEmail(mainDB, input.Email)
	if emailTaken {
		APIReturn(c, false, "the email address "+input.Email+" is already taken")
		return
	}

	user := DBCreateUser(mainDB, input.Name, input.Email, input.Password)
	if user.ID != 0 {
		SendValidationKey(user.ID, user.Email, user.ValidationKey)
		APIReturn(c, true, user)
	} else {
		APIReturn(c, false, "could not create user")
	}
}

func APICreatePost(c *gin.Context) {
	type Input struct {
		UserID   int64    `json:"userId"`
		Content  string   `json:"content"`
		Tags     []string `json:"tags"`
		Location []string `json:"location"`
	}
	var input Input

	if e := c.BindJSON(&input); DidFail(e, "get input for create post") {
		APIReturn(c, false, "invalid input values")
		return
	}

	if len(input.Location) == 0 {
		input.Location = getAddress(c.ClientIP())
	}
	date := time.Time{}
	post := DBCreatePost(mainDB, input.UserID, input.Content, date, input.Tags, input.Location)
	if post.ID != 0 {
		APIReturn(c, true, post)
	} else {
		APIReturn(c, false, "could not create post")
	}
}

func APICreateComment(c *gin.Context) {
	type Input struct {
		UserID  int64  `json:"userId"`
		ReplyID int64  `json:"replyId"`
		Content string `json:"content"`
	}
	var in Input

	if e := c.BindJSON(&in); DidFail(e, "get input for create comment") {
		APIReturn(c, false, "invalid input values")
		return
	}

	postId, e := strconv.ParseInt(c.Param("pid"), 10, 64)
	if DidFail(e, "invalid pid") {
		APIFailed(c, e, "invalid post id")
	}

	comment, _ := DBCreateComment(mainDB, in.UserID, in.Content, postId, in.ReplyID)
	if comment.ID != 0 {
		APIReturn(c, true, comment)
	} else {
		APIReturn(c, false, "could not create post")
	}
}

// ------------------------------------------------------------------------
// Delete
// ------------------------------------------------------------------------
func APIDeleteUser(c *gin.Context) {
	uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
	if APIFailed(c, e, "invalid user id") {
		return
	}

	DBDeleteUser(mainDB, uid)
	APIReturn(c, true, gin.H{})
}

func APIDeletePost(c *gin.Context) {
	pid, e := strconv.ParseInt(c.Param("pid"), 10, 64)

	if APIFailed(c, e, "invalid post id") {
		APIReturn(c, false, "invalid post id provided")
		return
	}

	DBDeletePost(mainDB, pid)
	APIReturn(c, true, gin.H{})
}

func APIDeleteComment(c *gin.Context) {
	pid, e := strconv.ParseInt(c.Param("pid"), 10, 64)
	if APIFailed(c, e, "invalid post id") {
		APIReturn(c, false, "invalid post id provided")
		return
	}

	cid, e := strconv.ParseInt(c.Param("cid"), 10, 64)
	if APIFailed(c, e, "invalid post id") {
		APIReturn(c, false, "invalid post id provided")
		return
	}

	DBDeleteComment(mainDB, pid, cid)
	APIReturn(c, true, gin.H{})
}

func APIDeleteUserContPlaylist(kind UserContKind) func(*gin.Context) {
	return func(c *gin.Context) {
		uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
		if APIFailed(c, e, "invalid user id") {
			APIReturn(c, false, "invalid user id")
			return
		}

		pid, e := strconv.ParseInt(c.Param("pid"), 10, 64)
		if APIFailed(c, e, "invalid post/user id") {
			APIReturn(c, false, "invalid post/user id")
			return
		}

		cont := DBDeleteUserCont(mainDB, kind, uid, pid, -1)
		APIReturn(c, true, cont)
	}
}

// ------------------------------------------------------------------------
// Get User
// ------------------------------------------------------------------------
func APIGetUser(c *gin.Context) {
	uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
	if APIFailed(c, e, "invalid user id") {
		return
	}

	user := DBGetUser(mainDB, uid, "")
	if user.ID == 0 {
		APIReturn(c, false, "no user found with id"+fmt.Sprint(uid))
	} else {
		APIReturn(c, true, user)
	}
}

func APIGetUsers(c *gin.Context) {
	upvotes, e := strconv.ParseInt(c.DefaultQuery("upvotes", "0"), 10, 64)
	if APIFailed(c, e, "invalid upvotes value given") {
		return
	}

	downvotes, e := strconv.ParseInt(c.DefaultQuery("downvotes", "0"), 10, 64)
	if APIFailed(c, e, "invalid downvotes value given") {
		return
	}

	order, e := SortOrderFromString(c.DefaultQuery("order", "score"))
	if APIFailed(c, e, "invalid order given") {
		return
	}

	offset, e := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
	if APIFailed(c, e, "invalid offset given") {
		return
	}

	limit, e := strconv.ParseInt(c.DefaultQuery("limit", "50"), 10, 64)
	if APIFailed(c, e, "invalid limit given") {
		return
	}

	popularIn := strings.Split(c.DefaultQuery("popularIn", ""), ",")
	if len(popularIn) == 1 && popularIn[0] == "" {
		popularIn = []string{}
	}

	startDate := c.DefaultQuery("start", "")
	endDate := c.DefaultQuery("end", "")

	forUser, e := strconv.ParseInt(c.DefaultQuery("for", "0"), 10, 64)
	if APIFailed(c, e, "invalid for user") {
		return
	}

	search := c.DefaultQuery("search", "")

	users := DBGetUsers(mainDB, popularIn, upvotes, downvotes, order, limit, offset, startDate, endDate, forUser, search)
	APIReturn(c, true, users)
}

func APIGetUserPrefs(kind UserPrefKind) func(*gin.Context) {
	return func(c *gin.Context) {
		upvotes, e := strconv.ParseInt(c.DefaultQuery("upvotes", "0"), 10, 64)
		if APIFailed(c, e, "invalid upvotes value given") {
			return
		}

		downvotes, e := strconv.ParseInt(c.DefaultQuery("downvotes", "0"), 10, 64)
		if APIFailed(c, e, "invalid downvotes value given") {
			return
		}

		order, e := SortOrderFromString(c.DefaultQuery("order", "score"))
		if APIFailed(c, e, "invalid order given") {
			return
		}

		offset, e := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
		if APIFailed(c, e, "invalid offset given") {
			return
		}

		limit, e := strconv.ParseInt(c.DefaultQuery("limit", "50"), 10, 64)
		if APIFailed(c, e, "invalid limit given") {
			return
		}

		uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
		if APIFailed(c, e, "invalid uid given") {
			return
		}

		pid, e := strconv.ParseInt(c.DefaultQuery("pid", "0"), 10, 64)
		if APIFailed(c, e, "invalid pid given") {
			return
		}

		sid, e := strconv.ParseInt(c.DefaultQuery("sid", "0"), 10, 64)
		if APIFailed(c, e, "invalid sid given") {
			return
		}

		prefs := DBGetUserPref(mainDB, uid, order, kind, pid, sid, upvotes, downvotes, limit, offset)
		APIReturn(c, true, prefs)
	}
}

func APIGetUserPrefUsers(c *gin.Context) {
	upvotes, e := strconv.ParseInt(c.DefaultQuery("upvotes", "0"), 10, 64)
	if APIFailed(c, e, "invalid upvotes value given") {
		return
	}

	downvotes, e := strconv.ParseInt(c.DefaultQuery("downvotes", "0"), 10, 64)
	if APIFailed(c, e, "invalid downvotes value given") {
		return
	}

	upvoteAmount, e := strconv.ParseInt(c.DefaultQuery("upvoteAmount", "0"), 10, 64)
	if APIFailed(c, e, "invalid upvote amount given") {
		return
	}

	downvoteAmount, e := strconv.ParseInt(c.DefaultQuery("downvoteAmount", "0"), 10, 64)
	if APIFailed(c, e, "invalid downvote amount given") {
		return
	}

	order, e := SortOrderFromString(c.DefaultQuery("order", "score"))
	if APIFailed(c, e, "invalid order given") {
		return
	}

	uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
	if APIFailed(c, e, "invalid uid given") {
		return
	}

	offset, e := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
	if APIFailed(c, e, "invalid offset given") {
		return
	}

	limit, e := strconv.ParseInt(c.DefaultQuery("limit", "50"), 10, 64)
	if APIFailed(c, e, "invalid limit given") {
		return
	}

	search := c.DefaultQuery("search", "")

	users := DBGetUserPrefUsers(mainDB, uid, upvoteAmount, downvoteAmount, upvotes, downvotes, order, search, limit, offset)
	APIReturn(c, true, users)
}

func APIGetUserPrefPosts(c *gin.Context) {
	author, e := strconv.ParseInt(c.DefaultQuery("uid", "0"), 10, 64)
	if APIFailed(c, e, "invalid user id") {
		return
	}

	location := strings.Split(c.DefaultQuery("location", ""), ",")
	if len(location) == 1 && location[0] == "" {
		location = []string{}
	}
	tags := []int64{}
	stringTags := strings.Split(c.DefaultQuery("tags", ""), ",")
	if !(len(stringTags) == 1 && stringTags[0] == "") {
		for _, s := range stringTags {
			t, e := strconv.ParseInt(s, 10, 64)
			if DidFail(e, "convert tag index") {
				continue
			}
			tags = append(tags, t)
		}
	}

	upvotes, e := strconv.ParseInt(c.DefaultQuery("upvotes", "0"), 10, 64)
	if APIFailed(c, e, "invalid upvotes value given") {
		return
	}

	downvotes, e := strconv.ParseInt(c.DefaultQuery("downvotes", "0"), 10, 64)
	if APIFailed(c, e, "invalid downvotes value given") {
		return
	}

	order, e := SortOrderFromString(c.DefaultQuery("order", "score"))
	if APIFailed(c, e, "invalid order given") {
		return
	}

	offset, e := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
	if APIFailed(c, e, "invalid offset given") {
		return
	}

	limit, e := strconv.ParseInt(c.DefaultQuery("limit", "50"), 10, 64)
	if APIFailed(c, e, "invalid limit given") {
		return
	}

	uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
	if APIFailed(c, e, "invalid uid given") {
		return
	}

	upvoteAmount, e := strconv.ParseInt(c.DefaultQuery("upvoteAmount", "0"), 10, 64)
	if APIFailed(c, e, "invalid upvote amount given") {
		return
	}

	downvoteAmount, e := strconv.ParseInt(c.DefaultQuery("downvoteAmount", "0"), 10, 64)
	if APIFailed(c, e, "invalid downvote amount given") {
		return
	}

	startDate := c.DefaultQuery("start", "")
	endDate := c.DefaultQuery("end", "")
	search := c.DefaultQuery("search", "")

	posts := DBGetUserPrefPosts(mainDB, uid, upvoteAmount, downvoteAmount, author, tags, location, upvotes, downvotes, order, search, limit, offset, startDate, endDate)
	APIReturn(c, true, posts)
}

func APIGetUserPrefComments(c *gin.Context) {
	replyId, e := strconv.ParseInt(c.DefaultQuery("replyId", "0"), 10, 64)
	if APIFailed(c, e, "invalid reply id given") {
		return
	}

	author, e := strconv.ParseInt(c.DefaultQuery("uid", "0"), 10, 64)
	if APIFailed(c, e, "invalid user id") {
		return
	}

	upvotes, e := strconv.ParseInt(c.DefaultQuery("upvotes", "0"), 10, 64)
	if APIFailed(c, e, "invalid upvotes value given") {
		return
	}

	downvotes, e := strconv.ParseInt(c.DefaultQuery("downvotes", "0"), 10, 64)
	if APIFailed(c, e, "invalid downvotes value given") {
		return
	}

	order, e := SortOrderFromString(c.DefaultQuery("order", "score"))
	if APIFailed(c, e, "invalid order given") {
		return
	}

	offset, e := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
	if APIFailed(c, e, "invalid offset given") {
		return
	}

	limit, e := strconv.ParseInt(c.DefaultQuery("limit", "50"), 10, 64)
	if APIFailed(c, e, "invalid limit given") {
		return
	}

	uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
	if APIFailed(c, e, "invalid uid given") {
		return
	}

	upvoteAmount, e := strconv.ParseInt(c.DefaultQuery("upvoteAmount", "0"), 10, 64)
	if APIFailed(c, e, "invalid upvote amount given") {
		return
	}

	downvoteAmount, e := strconv.ParseInt(c.DefaultQuery("downvoteAmount", "0"), 10, 64)
	if APIFailed(c, e, "invalid downvote amount given") {
		return
	}

	startDate := c.DefaultQuery("start", "")
	endDate := c.DefaultQuery("end", "")
	search := c.DefaultQuery("search", "")

	comments := DBGetUserPrefComments(mainDB, uid, upvoteAmount, downvoteAmount, author, replyId, upvotes, downvotes, order, search, limit, offset, startDate, endDate)
	APIReturn(c, true, comments)
}

func APIGetUserPrefTags(c *gin.Context) {
	uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
	if APIFailed(c, e, "invalid uid given") {
		return
	}

	upvoteAmount, e := strconv.ParseInt(c.DefaultQuery("upvoteAmount", "0"), 10, 64)
	if APIFailed(c, e, "invalid upvote amount given") {
		return
	}

	downvoteAmount, e := strconv.ParseInt(c.DefaultQuery("downvoteAmount", "0"), 10, 64)
	if APIFailed(c, e, "invalid downvote amount given") {
		return
	}

	location := strings.Split(c.DefaultQuery("location", ""), ",")
	if len(location) == 1 && location[0] == "" {
		location = []string{}
	}
	tags := strings.Split(c.DefaultQuery("tags", ""), ",")
	if len(tags) == 1 && tags[0] == "" {
		tags = []string{}
	}

	upvotes, e := strconv.ParseInt(c.DefaultQuery("upvotes", "0"), 10, 64)
	if APIFailed(c, e, "invalid upvotes value given") {
		return
	}

	downvotes, e := strconv.ParseInt(c.DefaultQuery("downvotes", "0"), 10, 64)
	if APIFailed(c, e, "invalid downvotes value given") {
		return
	}

	order, e := SortOrderFromString(c.DefaultQuery("order", "score"))
	if APIFailed(c, e, "invalid order given") {
		return
	}

	offset, e := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
	if APIFailed(c, e, "invalid offset given") {
		return
	}

	limit, e := strconv.ParseInt(c.DefaultQuery("limit", "50"), 10, 64)
	if APIFailed(c, e, "invalid limit given") {
		return
	}

	search := c.DefaultQuery("search", "")

	result := DBGetUserPrefTags(mainDB, uid, upvoteAmount, downvoteAmount, tags, location, upvotes, downvotes, order, search, limit, offset)
	APIReturn(c, true, result)
}

func APIGetUserContPost(kind UserContKind) func(*gin.Context) {
	return func(c *gin.Context) {
		uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
		if APIFailed(c, e, "invalid uid given") {
			return
		}

		location := strings.Split(c.DefaultQuery("location", ""), ",")
		if len(location) == 1 && location[0] == "" {
			location = []string{}
		}
		tags := []int64{}
		stringTags := strings.Split(c.DefaultQuery("tags", ""), ",")
		if !(len(stringTags) == 1 && stringTags[0] == "") {
			for _, s := range stringTags {
				t, e := strconv.ParseInt(s, 10, 64)
				if DidFail(e, "convert tag index") {
					continue
				}
				tags = append(tags, t)
			}
		}

		upvotes, e := strconv.ParseInt(c.DefaultQuery("upvotes", "0"), 10, 64)
		if APIFailed(c, e, "invalid upvotes value given") {
			return
		}

		downvotes, e := strconv.ParseInt(c.DefaultQuery("downvotes", "0"), 10, 64)
		if APIFailed(c, e, "invalid downvotes value given") {
			return
		}

		order, e := SortOrderFromString(c.DefaultQuery("order", "score"))
		if APIFailed(c, e, "invalid order given") {
			return
		}

		offset, e := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
		if APIFailed(c, e, "invalid offset given") {
			return
		}

		limit, e := strconv.ParseInt(c.DefaultQuery("limit", "50"), 10, 64)
		if APIFailed(c, e, "invalid limit given") {
			return
		}

		now := utc()
		lastWeek := now.AddDate(0, 0, -7)
		startDate := c.DefaultQuery("start", formatTime(lastWeek))
		endDate := c.DefaultQuery("end", formatTime(now))
		search := c.DefaultQuery("search", "")

		result := DBGetUserContPost(mainDB, kind, uid, tags, location, upvotes, downvotes, order, search, limit, offset, startDate, endDate)
		APIReturn(c, true, result)
	}
}

func APIGetUserContComments(c *gin.Context) {
	replyId, e := strconv.ParseInt(c.DefaultQuery("replyId", "0"), 10, 64)
	if APIFailed(c, e, "invalid reply id given") {
		return
	}

	author, e := strconv.ParseInt(c.DefaultQuery("uid", "0"), 10, 64)
	if APIFailed(c, e, "invalid user id") {
		return
	}

	upvotes, e := strconv.ParseInt(c.DefaultQuery("upvotes", "0"), 10, 64)
	if APIFailed(c, e, "invalid upvotes value given") {
		return
	}

	downvotes, e := strconv.ParseInt(c.DefaultQuery("downvotes", "0"), 10, 64)
	if APIFailed(c, e, "invalid downvotes value given") {
		return
	}

	order, e := SortOrderFromString(c.DefaultQuery("order", "score"))
	if APIFailed(c, e, "invalid order given") {
		return
	}

	offset, e := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
	if APIFailed(c, e, "invalid offset given") {
		return
	}

	limit, e := strconv.ParseInt(c.DefaultQuery("limit", "50"), 10, 64)
	if APIFailed(c, e, "invalid limit given") {
		return
	}

	uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
	if APIFailed(c, e, "invalid uid given") {
		return
	}

	startDate := c.DefaultQuery("start", "")
	endDate := c.DefaultQuery("end", "")
	search := c.DefaultQuery("search", "")

	comments := DBGetUserContComments(mainDB, uid, author, replyId, upvotes, downvotes, order, search, limit, offset, startDate, endDate)
	APIReturn(c, true, comments)
}

func APIGetUserContUsers(ucp UserContKind) func(*gin.Context) {
	return func(c *gin.Context) {
		uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
		if APIFailed(c, e, "invalid user id") {
			return
		}

		users := DBGetUserContUsers(mainDB, uid, ucp)
		APIReturn(c, true, users)
	}
}

// ------------------------------------------------------------------------
// Get Post
// ------------------------------------------------------------------------
func APIGetPost(c *gin.Context) {
	pid, e := strconv.ParseInt(c.Param("pid"), 10, 64)
	if APIFailed(c, e, "invalid post id") {
		return
	}

	post := DBGetPost(mainDB, pid)
	if post.ID != 0 {
		APIReturn(c, true, post)
	} else {
		APIReturn(c, false, "no post found with id "+fmt.Sprint(pid))
	}
}

func APIGetPosts(c *gin.Context) {
	uid, e := strconv.ParseInt(c.DefaultQuery("uid", "0"), 10, 64)
	if APIFailed(c, e, "invalid user id") {
		return
	}

	origin := strings.Split(c.DefaultQuery("origin", ""), ",")
	if len(origin) == 1 && origin[0] == "" {
		origin = []string{}
	}

	popularIn := strings.Split(c.DefaultQuery("popularIn", ""), ",")
	if len(popularIn) == 1 && popularIn[0] == "" {
		popularIn = []string{}
	}

	tags := []int64{}
	stringTags := strings.Split(c.DefaultQuery("tags", ""), ",")
	if !(len(stringTags) == 1 && stringTags[0] == "") {
		for _, s := range stringTags {
			t, e := strconv.ParseInt(s, 10, 64)
			if DidFail(e, "convert tag index") {
				continue
			}
			tags = append(tags, t)
		}
	}

	upvotes, e := strconv.ParseInt(c.DefaultQuery("upvotes", "0"), 10, 64)
	if APIFailed(c, e, "invalid upvotes value given") {
		return
	}

	downvotes, e := strconv.ParseInt(c.DefaultQuery("downvotes", "0"), 10, 64)
	if APIFailed(c, e, "invalid downvotes value given") {
		return
	}

	order, e := SortOrderFromString(c.DefaultQuery("order", "score"))
	if APIFailed(c, e, "invalid order given") {
		return
	}

	offset, e := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
	if APIFailed(c, e, "invalid offset given") {
		return
	}

	limit, e := strconv.ParseInt(c.DefaultQuery("limit", "50"), 10, 64)
	if APIFailed(c, e, "invalid limit given") {
		return
	}

	start := c.DefaultQuery("startCreated", "")
	end := c.DefaultQuery("endCreated", "")
	startDate := c.DefaultQuery("start", "")
	endDate := c.DefaultQuery("end", "")

	forUser, e := strconv.ParseInt(c.DefaultQuery("for", "0"), 10, 64)
	if APIFailed(c, e, "invalid for user") {
		return
	}

	search := c.DefaultQuery("search", "")

	posts := DBGetPosts(mainDB, uid, tags, origin, popularIn, upvotes, downvotes, order, limit, offset, start, end, startDate, endDate, forUser, search)
	APIReturn(c, true, posts)
}

// ------------------------------------------------------------------------
// Get Comment
// ------------------------------------------------------------------------
func APIGetComment(c *gin.Context) {
	pid, e := strconv.ParseInt(c.Param("pid"), 10, 64)
	if APIFailed(c, e, "invalid post id") {
		return
	}

	cid, e := strconv.ParseInt(c.Param("cid"), 10, 64)
	if APIFailed(c, e, "invalid comment id") {
		return
	}

	result := DBGetComment(mainDB, pid, cid)
	APIReturn(c, true, result)
}

func APIGetComments(c *gin.Context) {
	uid, e := strconv.ParseInt(c.DefaultQuery("uid", "0"), 10, 64)
	if APIFailed(c, e, "invalid user id") {
		return
	}

	pid, e := strconv.ParseInt(c.DefaultQuery("pid", "0"), 10, 64)
	if APIFailed(c, e, "invalid post id") {
		return
	}

	replyId, e := strconv.ParseInt(c.DefaultQuery("reply", "-1"), 10, 64)
	if APIFailed(c, e, "invalid reply id") {
		return
	}

	upvotes, e := strconv.ParseInt(c.DefaultQuery("upvotes", "0"), 10, 64)
	if APIFailed(c, e, "invalid upvotes value given") {
		return
	}

	downvotes, e := strconv.ParseInt(c.DefaultQuery("downvotes", "0"), 10, 64)
	if APIFailed(c, e, "invalid downvotes value given") {
		return
	}

	order, e := SortOrderFromString(c.DefaultQuery("order", "score"))
	if APIFailed(c, e, "invalid order given") {
		return
	}

	offset, e := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
	if APIFailed(c, e, "invalid offset given") {
		return
	}

	limit, e := strconv.ParseInt(c.DefaultQuery("limit", "50"), 10, 64)
	if APIFailed(c, e, "invalid limit given") {
		return
	}

	popularIn := strings.Split(c.DefaultQuery("popularIn", ""), ",")
	if len(popularIn) == 1 && popularIn[0] == "" {
		popularIn = []string{}
	}

	startCreated := c.DefaultQuery("startCreated", "")
	endCreated := c.DefaultQuery("endCreated", "")
	start := c.DefaultQuery("start", "")
	end := c.DefaultQuery("end", "")

	forUser, e := strconv.ParseInt(c.DefaultQuery("for", "0"), 10, 64)
	if APIFailed(c, e, "invalid for user") {
		return
	}

	search := c.DefaultQuery("search", "")

	result := DBGetComments(mainDB, pid, uid, replyId, start, end, popularIn, upvotes, downvotes, order, limit, offset, startCreated, endCreated, forUser, search)
	APIReturn(c, true, result)
}

// ------------------------------------------------------------------------
// Get Tag
// ------------------------------------------------------------------------
func APIGetTag(c *gin.Context) {
	tid, e := strconv.ParseInt(c.Param("tid"), 10, 64)
	if APIFailed(c, e, "invalid tag id") {
		return
	}

	tag := DBGetTags(mainDB, tid, []string{}, []string{}, 0, 0, soScore, 1, 0, "", "", 0, "")
	if len(tag) == 1 {
		APIReturn(c, true, tag[0])
	} else {
		APIReturn(c, false, "no tag found with id "+fmt.Sprint(tid))
	}
}

func APIGetTagsFromIDs(c *gin.Context) {
	type Input struct {
		Ids []int64 `json:"ids"`
	}
	var in Input

	if e := c.BindJSON(&in); DidFail(e, "get input for tags") {
		APIReturn(c, false, "invalid input values")
		return
	}

	result := DBGetTagsFromIDs(mainDB, in.Ids)

	APIReturn(c, true, result)
}

func APIGetTags(c *gin.Context) {
	location := strings.Split(c.DefaultQuery("location", ""), ",")
	if len(location) == 1 && location[0] == "" {
		location = []string{}
	}
	tags := strings.Split(c.DefaultQuery("tags", ""), ",")
	if len(tags) == 1 && tags[0] == "" {
		tags = []string{}
	}

	upvotes, e := strconv.ParseInt(c.DefaultQuery("upvotes", "0"), 10, 64)
	if APIFailed(c, e, "invalid upvotes value given") {
		return
	}

	downvotes, e := strconv.ParseInt(c.DefaultQuery("downvotes", "0"), 10, 64)
	if APIFailed(c, e, "invalid downvotes value given") {
		return
	}

	order, e := SortOrderFromString(c.DefaultQuery("order", "score"))
	if APIFailed(c, e, "invalid order given") {
		return
	}

	offset, e := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
	if APIFailed(c, e, "invalid offset given") {
		return
	}

	limit, e := strconv.ParseInt(c.DefaultQuery("limit", "50"), 10, 64)
	if APIFailed(c, e, "invalid limit given") {
		return
	}

	startDate := c.DefaultQuery("start", "")
	endDate := c.DefaultQuery("end", "")

	forUser, e := strconv.ParseInt(c.DefaultQuery("for", "0"), 10, 64)
	if APIFailed(c, e, "invalid for user") {
		return
	}

	search := c.DefaultQuery("search", "")

	result := DBGetTags(mainDB, 0, tags, location, upvotes, downvotes, order, limit, offset, startDate, endDate, forUser, search)
	APIReturn(c, true, result)
}

// ------------------------------------------------------------------------
// Voting
// ------------------------------------------------------------------------

func APIVoteUser(c *gin.Context) {
	targetId, e := strconv.ParseInt(c.Param("uid"), 10, 64)
	if APIFailed(c, e, "invalid target user id") {
		return
	}

	type Input struct {
		UID    int64 `json:"uid"`
		Amount int64 `json:"amount"`
	}
	var input Input
	if e := c.BindJSON(&input); DidFail(e, "get input for vote post") {
		APIFailed(c, e, "invalid input values")
		return
	}

	if !DBCanUpdateCredit(mainDB, input.UID, input.Amount) {
		APIFailed(c, errors.New(""), "not enough credits")
		return
	}

	addr := getAddress(c.ClientIP())

	DBVoteForUser(mainDB, input.UID, targetId, input.Amount, addr, "")
	remaining := DBSubtractUserCredit(mainDB, input.UID, input.Amount)

	if remaining >= 0 {
		APIReturn(c, true, remaining)
	} else {
		APIFailed(c, errors.New(""), "could not complete vote")
	}
}

func APIVotePost(c *gin.Context) {
	pid, e := strconv.ParseInt(c.Param("pid"), 10, 64)
	if APIFailed(c, e, "invalid target user id") {
		return
	}

	type Input struct {
		UID    int64 `json:"uid"`
		Amount int64 `json:"amount"`
	}
	var input Input
	if e := c.BindJSON(&input); DidFail(e, "get input for vote post") {
		APIFailed(c, e, "invalid input values")
		return
	}

	if !DBCanUpdateCredit(mainDB, input.UID, input.Amount) {
		APIFailed(c, errors.New(""), "not enough credits")
		return
	}

	addr := getAddress(c.ClientIP())

	DBVotePost(mainDB, input.UID, pid, input.Amount, addr, "")
	remaining := DBSubtractUserCredit(mainDB, input.UID, input.Amount)

	if remaining >= 0 {
		APIReturn(c, true, remaining)
	} else {
		APIFailed(c, errors.New(""), "could not complete vote insufficient credit")
	}
}

func APIVoteComment(c *gin.Context) {
	pid, e := strconv.ParseInt(c.Param("pid"), 10, 64)
	if APIFailed(c, e, "invalid target user id") {
		return
	}

	cid, e := strconv.ParseInt(c.Param("cid"), 10, 64)
	if APIFailed(c, e, "invalid target user id") {
		return
	}

	type Input struct {
		UID    int64 `json:"uid"`
		Amount int64 `json:"amount"`
	}
	var input Input
	if e := c.BindJSON(&input); DidFail(e, "get input for vote post") {
		APIFailed(c, e, "invalid input values")
		return
	}

	if !DBCanUpdateCredit(mainDB, input.UID, input.Amount) {
		APIFailed(c, errors.New(""), "not enough credits")
		return
	}

	addr := getAddress(c.ClientIP())

	DBVoteComment(mainDB, input.UID, pid, cid, input.Amount, addr, "")
	remaining := DBSubtractUserCredit(mainDB, input.UID, input.Amount)

	if remaining >= 0 {
		APIReturn(c, true, remaining)
	} else {
		APIFailed(c, errors.New(""), "could not complete vote")
	}
}

func APIAddUserCont(kind UserContKind) func(*gin.Context) {
	return func(c *gin.Context) {
		uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
		if APIFailed(c, e, "invalid user id") {
			APIReturn(c, false, "invalid user id")
			return
		}

		type Input struct {
			PID int64 `json:"pid"`
		}
		var in Input
		if e := c.BindJSON(&in); DidFail(e, "get input for user cont") {
			APIFailed(c, e, "invalid input values")
			return
		}

		cont := DBCreateUserCont(mainDB, kind, uid, in.PID, -1)
		APIReturn(c, true, cont)
	}
}

func APIRefreshUserContRecommendations(c *gin.Context) {
	uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
	if APIFailed(c, e, "invalid user id") {
		APIReturn(c, false, "invalid user id")
		return
	}

	type Input struct {
		PIDs []int64 `json:"pid"`
	}
	var in Input
	if e := c.BindJSON(&in); DidFail(e, "get input for user cont") {
		APIFailed(c, e, "invalid input values")
		return
	}

	DBRefreshUserContRecommended(mainDB, uid, in.PIDs)
}

func APIPurchaseCredit(c *gin.Context) {
	type Input struct {
		Amount int64 `json:"amount"`
	}
	var input Input
	if e := c.BindJSON(&input); DidFail(e, "get input for vote post") {
		APIFailed(c, e, "invalid input values")
		return
	}

	targetId, e := strconv.ParseInt(c.Param("uid"), 10, 64)
	if APIFailed(c, e, "invalid target user id") {
		return
	}

	newAmount := DBAddUserCredit(mainDB, targetId, input.Amount)

	if newAmount >= 0 {
		APIReturn(c, true, gin.H{"credits_remaining": newAmount})
	} else {
		APIFailed(c, errors.New(""), "could not complete top up")
	}
}

func APIWatchUser(c *gin.Context) {
	uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
	if APIFailed(c, e, "invalid user id") {
		return
	}

	type Input struct {
		Tags []int64 `json:"tags"`
		Time float64 `json:"time"`
	}
	var input Input
	if e := c.BindJSON(&input); APIFailed(c, e, "get input for vote post") {
		return
	}

	pref := DBWatchUser(mainDB, uid, input.Tags, input.Time)

	APIReturn(c, true, pref)
}

// ------------------------------------------------------------------------
// Admin
// ------------------------------------------------------------------------

func APICreateFlag(c *gin.Context) {
	type Input struct {
		UID    int64      `json:"uid"`
		PID    int64      `json:"pid"`
		SID    int64      `json:"sid"`
		Kind   FlagReason `json:"kind"`
		Reason string     `json:"reason"`
	}

	var in Input
	if e := c.BindJSON(&in); APIFailed(c, e, "get input for flag") {
		return
	}

	DBCreateFlag(mainDB, in.UID, in.PID, in.SID, in.Kind, in.Reason)

	APIReturn(c, true, gin.H{})
}

func APIGetFlags(c *gin.Context) {
	kind, e := strconv.ParseInt(c.DefaultQuery("kind", fmt.Sprint(frSpam)), 10, 64)
	if APIFailed(c, e, "invalid flag kind") {
		return
	}

	limit, e := strconv.ParseInt(c.DefaultQuery("limit", "50"), 10, 64)
	if APIFailed(c, e, "invalid limit value") {
		return
	}

	offset, e := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
	if APIFailed(c, e, "invalid offset value") {
		return
	}

	content := DBGetFlags(mainDB, FlagReason(kind), limit, offset)

	APIReturn(c, true, content)
}

func APIHandleFlag(c *gin.Context) {
	type Input struct {
		PID    int64  `json:"pid"`
		SID    int64  `json:"sid"`
		Action string `json:"action"`
	}

	var in Input
	if e := c.BindJSON(&in); APIFailed(c, e, "get input for flag") {
		return
	}

	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if APIFailed(c, e, "invalid flag id") {
		return
	}

	DBHandleFlag(mainDB, id, in.PID, in.SID, in.Action)

	APIReturn(c, true, gin.H{})
}

// ------------------------------------------------------------------------
// Sign In
// ------------------------------------------------------------------------
func APISignIn(c *gin.Context) {
	type Input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	var in Input
	if e := c.BindJSON(&in); DidFail(e, "get input for sign in") {
		APIReturn(c, false, "invalid input values")
		return
	}

	user := DBSignIn(mainDB, in.Email, in.Password)
	if user.ID != 0 {
		// FIXME: return proper token which is stored on the server
		// to cross reference with user to ensure user updates only their data
		APIReturn(c, true, gin.H{"user": user, "token": "secret"})
	} else {
		APIReturn(c, false, "password or email incorrect")
	}
}

func APISignInWithApple(c *gin.Context) {
	teamID := "86QZ48F54E"
	serviceID := "com.letanyan.newsourceserviceid"
	keyID := "K3NQ5VC2LH"
	// bundleID := "com.letanyan.newsource"
	secretFile := `-----BEGIN PRIVATE KEY-----
MIGTAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBHkwdwIBAQQgX8e/+ExMOMTbLzav
lg8rFYOhBfeGrcAIKL+7Q4FjjSGgCgYIKoZIzj0DAQehRANCAATtI0L8/MPp2b4T
J6/1jA9dnkP0SodRODScM2opvHJgKYhevNPi/Blu5pd3ble2zGBctKdDbHpW6Xf3
jAhjLaPD
-----END PRIVATE KEY-----`

	// Generate the client secret used to authenticate with Apple's validation servers
	// Refer to the example files to see where to get secret, teamID, clientID, keyID
	secret, _ := apple.GenerateClientSecret(secretFile, teamID, serviceID, keyID)

	// Generate a new validation client
	client := apple.New()

	vReq := apple.AppValidationTokenRequest{
		ClientID:     serviceID,
		ClientSecret: secret,
		Code:         "the_token_to_validate",
	}

	var resp apple.ValidationResponse

	// Do the verification
	client.VerifyAppToken(context.Background(), vReq, &resp)

	unique, _ := apple.GetUniqueID(resp.IDToken)

	// Voila!
	fmt.Println(unique)
}

func APIResendVerificationLink(c *gin.Context) {
	type Input struct {
		Email string `json:"email"`
		Key   int32  `json:"key"`
	}

	var in Input
	if e := c.BindJSON(&in); APIFailed(c, e, "get input for verification link") {
		return
	}

	uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
	if APIFailed(c, e, "invalid user id") {
		APIReturn(c, false, "invalid user id")
		return
	}

	SendValidationKey(uid, in.Email, in.Key)
	APIReturn(c, true, 0)
}

func APIVerifyUserEmail(c *gin.Context) {
	uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
	if APIFailed(c, e, "invalid user id") {
		APIReturn(c, false, "invalid user id")
		return
	}

	key, e := strconv.ParseInt(c.Param("key"), 10, 32)
	if APIFailed(c, e, "invalid key") {
		APIReturn(c, false, "invalid key")
		return
	}

	res := DBValidateUser(mainDB, uid, int32(key))
	if res {
		APIReturn(c, true, 1)
	} else {
		APIReturn(c, false, "invalid verification key")
	}
}
