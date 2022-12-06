package main

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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

func APIReturnHTML(c *gin.Context, file string, obj any) {
	c.Header("Access-Control-Allow-Origin", "*")         // Required for CORS support to work
	c.Header("Access-Control-Allow-Credentials", "true") // Required for cookies, authorization headers with HTTPS
	c.Header("Access-Control-Allow-Headers", "Origin,Content-Type,X-Amz-Date,Authorization,X-Api-Key,X-Amz-Security-Token,locale")
	c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE")
	c.HTML(http.StatusOK, file, obj)
}

func APIFailed(c *gin.Context, e error, reason string) bool {
	if e == nil {
		return false
	} else {
		APIReturn(c, false, reason)
		return true
	}
}

func ContextMatchSecret(c *gin.Context, user int64) bool {
	secret := c.DefaultQuery("secret", "")
	deviceId, _ := url.QueryUnescape(c.DefaultQuery("device", ""))
	return AUTHMatchSecret(mainDB, user, deviceId, secret)
}

func APIMatchSecret(c *gin.Context, user int64) bool {
	if ContextMatchSecret(c, user) {
		return true
	} else {
		APIReturn(c, false, "Re-sign in to refresh session")
		return false
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
		Device   string `json:"device"`
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
	if user.ID > 0 {
		MailValidationKey(user.ID, user.Email, user.ValidationKey)
		secret := AUTHRegister(mainDB, user.ID, input.Device)
		APIReturn(c, true, gin.H{
			"user":      user,
			"token":     secret,
			"streak":    user.Credits,
			"following": []string{},
			"ignored":   []string{},
			"tags":      []string{},
		})
	} else {
		APIReturn(c, false, "could not create user")
	}
}

func APICreatePost(c *gin.Context) {
	type Input struct {
		UserID    int64    `json:"userId"`
		Content   string   `json:"content"`
		Location  []string `json:"location"`
		IsPreview bool     `json:"isPreview"`
		Locale    string   `json:"locale"`
	}
	var in Input

	if e := c.BindJSON(&in); DidFail(e, "get input for create post") {
		APIReturn(c, false, "invalid input values")
		return
	}

	if len(in.Content) > 10000 {
		APIReturn(c, false, "message content too long")
	}

	if !APIMatchSecret(c, in.UserID) {
		return
	}

	if in.IsPreview {
		// assume the client verified that content is just a url
		// and wants to use server for producing a preview.
		// In other words `IsPreview` can only be true if `Content`
		// is a single url.
		scrape := NAScrapeWebsite(in.Content)
		result := NACreatePost(in.UserID, in.Content, in.Locale, scrape, false)
		APIReturn(c, true, result)
	} else {
		user, _ := DBGetUser(mainDB, in.UserID, "")
		if user.Blocked.After(utc()) {
			APIReturn(c, false, fmt.Sprintf("you are blocked until %s", formatDate(user.Blocked)))
		} else {
			date := time.Time{}
			text, tags := DBPrepareTaggedString(in.Content, true)
			post := DBCreatePost(mainDB, in.UserID, text, date, tags, in.Location, in.Locale)
			if post.ID > 0 {
				APIReturn(c, true, post)
			} else {
				APIReturn(c, false, "could not create post")
			}
		}

	}
}

func APICreateComment(c *gin.Context) {
	type Input struct {
		UserID   int64  `json:"userId"`
		ReplyID  int64  `json:"replyId"`
		Content  string `json:"content"`
		IsReview bool   `json:"isReview"`
		Locale   string `json:"locale"`
	}
	var in Input

	if e := c.BindJSON(&in); DidFail(e, "get input for create comment") {
		APIReturn(c, false, "invalid input values")
		return
	}

	if in.IsReview && len(in.Content) > 10000 {
		APIReturn(c, false, "message content too long")
	}
	if !in.IsReview && len(in.Content) > 1000 {
		APIReturn(c, false, "message content too long")
	}

	if !APIMatchSecret(c, in.UserID) {
		return
	}

	postId, e := strconv.ParseInt(c.Param("pid"), 10, 64)
	if APIFailed(c, e, "invalid post id") {
		return
	}

	user, _ := DBGetUser(mainDB, in.UserID, "")
	if user.Blocked.After(utc()) {
		APIReturn(c, false, fmt.Sprintf("you are blocked until %s", formatDate(user.Blocked)))
	} else {
		comment, _ := DBCreateComment(mainDB, in.UserID, in.Content, postId, in.ReplyID, in.IsReview, in.Locale)
		if comment.ID > 0 {
			APIReturn(c, true, comment)
		} else {
			APIReturn(c, false, "could not create post")
		}
	}
}

func APIUpdatePost(c *gin.Context) {
	type Input struct {
		UserID  int64  `json:"userId"`
		Content string `json:"content"`
	}
	var in Input

	if e := c.BindJSON(&in); DidFail(e, "get input for update post") {
		APIReturn(c, false, "invalid input values")
		return
	}

	if !APIMatchSecret(c, in.UserID) {
		return
	}

	postId, e := strconv.ParseInt(c.Param("pid"), 10, 64)
	if APIFailed(c, e, "invalid post id") {
		return
	}

	user, _ := DBGetUser(mainDB, in.UserID, "")
	if user.Blocked.After(utc()) {
		APIReturn(c, false, fmt.Sprintf("you are blocked until %s", formatDate(user.Blocked)))
	} else {
		post := DBUpdatePost(mainDB, postId, in.Content)
		APIReturn(c, true, post)
	}
}

func APIUpdateComment(c *gin.Context) {
	type Input struct {
		UserID  int64  `json:"userId"`
		Content string `json:"content"`
	}
	var in Input

	if e := c.BindJSON(&in); DidFail(e, "get input for update comment") {
		APIReturn(c, false, "invalid input values")
		return
	}

	if !APIMatchSecret(c, in.UserID) {
		return
	}

	postId, e := strconv.ParseInt(c.Param("pid"), 10, 64)
	if APIFailed(c, e, "invalid post id") {
		return
	}

	commentId, e := strconv.ParseInt(c.Param("cid"), 10, 64)
	if APIFailed(c, e, "invalid post id") {
		return
	}

	user, _ := DBGetUser(mainDB, in.UserID, "")
	if user.Blocked.After(utc()) {
		APIReturn(c, false, fmt.Sprintf("you are blocked until %s", formatDate(user.Blocked)))
	} else {
		comment := DBUpdateComment(mainDB, postId, commentId, in.Content)
		APIReturn(c, true, comment)
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

	if !APIMatchSecret(c, uid) {
		return
	}

	DBDeleteUser(mainDB, uid)
	APIReturn(c, true, gin.H{})
}

func APIDeletePost(c *gin.Context) {
	pid, e := strconv.ParseInt(c.Param("pid"), 10, 64)
	if APIFailed(c, e, "invalid post id") {
		return
	}

	post := DBGetPost(mainDB, pid)
	if !APIMatchSecret(c, post.Author.ID) {
		return
	}

	DBDeletePost(mainDB, pid)
	APIReturn(c, true, gin.H{})
}

func APIDeleteComment(c *gin.Context) {
	pid, e := strconv.ParseInt(c.Param("pid"), 10, 64)
	if APIFailed(c, e, "invalid post id provided") {
		return
	}

	cid, e := strconv.ParseInt(c.Param("cid"), 10, 64)
	if APIFailed(c, e, "invalid comment id provided") {
		return
	}

	comment := DBGetComment(mainDB, pid, cid)
	isCommenter := ContextMatchSecret(c, comment.Author.ID)
	if !isCommenter {
		post := DBGetPost(mainDB, pid)
		isPoster := ContextMatchSecret(c, post.Author.ID)
		if !isPoster {
			APIReturn(c, false, "Re-sign in to refresh session")
			return
		}
	}

	DBDeleteComment(mainDB, pid, cid)
	APIReturn(c, true, gin.H{})
}

func APIDeleteUserContPlaylist(kind UserContKind) func(*gin.Context) {
	return func(c *gin.Context) {
		uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
		if APIFailed(c, e, "invalid user id") {
			return
		}

		if !APIMatchSecret(c, uid) {
			return
		}

		pid, e := strconv.ParseInt(c.Param("pid"), 10, 64)
		if APIFailed(c, e, "invalid post/user id") {
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

	if !APIMatchSecret(c, uid) {
		return
	}

	user, streak := DBGetUser(mainDB, uid, "")
	if user.ID <= 0 {
		APIReturn(c, false, "no user found with id "+fmt.Sprint(uid))
	} else {
		following := DBGetUserContUsers(mainDB, true, user.ID, ucpUserFollow, 0, 0, "", "")
		ignored := DBGetUserContUsers(mainDB, true, user.ID, ucpUserIgnored, 0, 0, "", "")
		tagFollowing := DBGetUserContTag(mainDB, true, user.ID, ucpTagFollow, 0, 0, "", "")
		APIReturn(c, true, gin.H{"user": user, "token": "notSecret", "streak": streak, "following": following, "ignored": ignored, "tags": tagFollowing})
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

	startDate := sanitizeDate(c.DefaultQuery("start", ""))
	endDate := sanitizeDate(c.DefaultQuery("end", ""))

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
		isOwner := ContextMatchSecret(c, uid)

		prefs := DBGetUserPref(mainDB, isOwner, uid, order, kind, pid, sid, upvotes, downvotes, limit, offset)
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

	startDate := sanitizeDate(c.DefaultQuery("start", ""))
	endDate := sanitizeDate(c.DefaultQuery("end", ""))
	search := c.DefaultQuery("search", "")
	isOwner := ContextMatchSecret(c, uid)

	users := DBGetUserPrefUsers(mainDB, isOwner, uid, upvoteAmount, downvoteAmount, upvotes, downvotes, order, search, limit, offset, startDate, endDate)
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

	startDate := sanitizeDate(c.DefaultQuery("start", ""))
	endDate := sanitizeDate(c.DefaultQuery("end", ""))
	search := c.DefaultQuery("search", "")

	isOwner := ContextMatchSecret(c, uid)

	posts := DBGetUserPrefPosts(mainDB, isOwner, uid, upvoteAmount, downvoteAmount, author, tags, location, upvotes, downvotes, order, search, limit, offset, startDate, endDate)
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

	startDate := sanitizeDate(c.DefaultQuery("start", ""))
	endDate := sanitizeDate(c.DefaultQuery("end", ""))
	search := c.DefaultQuery("search", "")

	isOwner := ContextMatchSecret(c, uid)

	comments := DBGetUserPrefComments(mainDB, isOwner, uid, upvoteAmount, downvoteAmount, author, replyId, upvotes, downvotes, order, search, limit, offset, startDate, endDate)
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

	startDate := sanitizeDate(c.DefaultQuery("start", ""))
	endDate := sanitizeDate(c.DefaultQuery("end", ""))
	search := c.DefaultQuery("search", "")
	isOwner := ContextMatchSecret(c, uid)

	result := DBGetUserPrefTags(mainDB, isOwner, uid, upvoteAmount, downvoteAmount, tags, upvotes, downvotes, order, search, limit, offset, startDate, endDate)
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

		startDate := sanitizeDate(c.DefaultQuery("start", ""))
		endDate := sanitizeDate(c.DefaultQuery("end", ""))
		search := c.DefaultQuery("search", "")
		isOwner := ContextMatchSecret(c, uid)

		result := DBGetUserContPost(mainDB, isOwner, kind, uid, tags, location, upvotes, downvotes, order, search, limit, offset, startDate, endDate)
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

	startDate := sanitizeDate(c.DefaultQuery("start", ""))
	endDate := sanitizeDate(c.DefaultQuery("end", ""))
	search := c.DefaultQuery("search", "")
	isReview := c.DefaultQuery("isReview", "0") == "1"

	comments := DBGetUserContComments(mainDB, uid, author, replyId, isReview, upvotes, downvotes, order, search, limit, offset, startDate, endDate)
	APIReturn(c, true, comments)
}

func APIGetUserContUsers(ucp UserContKind) func(*gin.Context) {
	return func(c *gin.Context) {
		uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
		if APIFailed(c, e, "invalid user id") {
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
		isOwner := ContextMatchSecret(c, uid)
		startDate := sanitizeDate(c.DefaultQuery("start", ""))
		endDate := sanitizeDate(c.DefaultQuery("end", ""))

		users := DBGetUserContUsers(mainDB, isOwner, uid, ucp, limit, offset, startDate, endDate)
		APIReturn(c, true, users)
	}
}

func APIGetUserContTags(ucp UserContKind) func(*gin.Context) {
	return func(c *gin.Context) {
		uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
		if APIFailed(c, e, "invalid user id") {
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
		isOwner := ContextMatchSecret(c, uid)
		startDate := sanitizeDate(c.DefaultQuery("start", ""))
		endDate := sanitizeDate(c.DefaultQuery("end", ""))

		tags := DBGetUserContTag(mainDB, isOwner, uid, ucp, limit, offset, startDate, endDate)
		APIReturn(c, true, tags)
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
	if post.ID > 0 {
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

	start := sanitizeDate(c.DefaultQuery("startCreated", ""))
	end := sanitizeDate(c.DefaultQuery("endCreated", ""))
	startDate := sanitizeDate(c.DefaultQuery("start", ""))
	endDate := sanitizeDate(c.DefaultQuery("end", ""))

	forUser, e := strconv.ParseInt(c.DefaultQuery("for", "0"), 10, 64)
	if APIFailed(c, e, "invalid for user") {
		return
	}
	if forUser != 0 {
		if !APIMatchSecret(c, forUser) {
			return
		}
	}

	search := c.DefaultQuery("search", "")

	posts := DBGetPosts(mainDB, uid, tags, origin, popularIn, upvotes, downvotes, order, limit, offset, start, end, startDate, endDate, forUser, search)
	APIReturn(c, true, posts)
}

func APIGetSimilarPosts(c *gin.Context) {
	uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
	if APIFailed(c, e, "invalid user id") {
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

	// startDate := sanitizeDate(c.DefaultQuery("start", ""))
	// endDate := sanitizeDate(c.DefaultQuery("end", ""))

	pid, e := strconv.ParseInt(c.DefaultQuery("pid", "0"), 10, 64)
	if APIFailed(c, e, "invalid post id") {
		return
	}

	posts := DBGetSimilarPosts(mainDB, uid, pid, order, limit, offset)
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

	startCreated := sanitizeDate(c.DefaultQuery("startCreated", ""))
	endCreated := sanitizeDate(c.DefaultQuery("endCreated", ""))
	start := sanitizeDate(c.DefaultQuery("start", ""))
	end := sanitizeDate(c.DefaultQuery("end", ""))
	isReview, e := strconv.ParseInt(c.DefaultQuery("isReview", "0"), 10, 64)
	if APIFailed(c, e, "isReview must be int") {
		return
	}

	forUser, e := strconv.ParseInt(c.DefaultQuery("for", "0"), 10, 64)
	if APIFailed(c, e, "invalid for user") {
		return
	}

	search := c.DefaultQuery("search", "")

	result := DBGetComments(mainDB, pid, uid, replyId, int8(isReview), start, end, popularIn, upvotes, downvotes, order, limit, offset, startCreated, endCreated, forUser, search)
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
	if len(in.Ids) == 0 {
		APIReturn(c, true, []Tag{})
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

	startDate := sanitizeDate(c.DefaultQuery("start", ""))
	endDate := sanitizeDate(c.DefaultQuery("end", ""))

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
		UID      int64    `json:"uid"`
		Amount   int64    `json:"amount"`
		Location []string `json:"location"`
	}
	var in Input
	if e := c.BindJSON(&in); DidFail(e, "get input for vote post") {
		APIFailed(c, e, "invalid input values")
		return
	}

	if !APIMatchSecret(c, in.UID) {
		return
	}

	if !DBCanUpdateCredit(mainDB, in.UID, in.Amount) {
		APIFailed(c, errors.New(""), "not enough credits")
		return
	}

	DBVoteForUser(mainDB, in.UID, targetId, in.Amount, in.Location)
	remaining := DBSubtractUserCredit(mainDB, in.UID, in.Amount)

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
		UID      int64    `json:"uid"`
		Amount   int64    `json:"amount"`
		Location []string `json:"location"`
	}
	var in Input
	if e := c.BindJSON(&in); DidFail(e, "get input for vote post") {
		APIFailed(c, e, "invalid input values")
		return
	}

	if !APIMatchSecret(c, in.UID) {
		return
	}

	if !DBCanUpdateCredit(mainDB, in.UID, in.Amount) {
		APIFailed(c, errors.New(""), "not enough credits")
		return
	}

	DBVotePost(mainDB, in.UID, pid, in.Amount, in.Location)
	if in.Amount < 0 {
		in.Amount = -in.Amount
	}
	remaining := DBSubtractUserCredit(mainDB, in.UID, in.Amount)

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
		UID      int64    `json:"uid"`
		Amount   int64    `json:"amount"`
		Location []string `json:"location"`
	}
	var in Input
	if e := c.BindJSON(&in); DidFail(e, "get input for vote post") {
		APIFailed(c, e, "invalid input values")
		return
	}

	if !APIMatchSecret(c, in.UID) {
		return
	}

	if !DBCanUpdateCredit(mainDB, in.UID, in.Amount) {
		APIFailed(c, errors.New(""), "not enough credits")
		return
	}

	DBVoteComment(mainDB, in.UID, pid, cid, in.Amount, in.Location)
	if in.Amount < 0 {
		in.Amount = -in.Amount
	}
	remaining := DBSubtractUserCredit(mainDB, in.UID, in.Amount)

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
		if !APIMatchSecret(c, uid) {
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
	if !APIMatchSecret(c, uid) {
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
	if !APIMatchSecret(c, targetId) {
		return
	}

	newAmount := DBAddUserCredit(mainDB, targetId, input.Amount)

	if newAmount >= 0 {
		APIReturn(c, true, newAmount)
	} else {
		APIFailed(c, errors.New(""), "could not complete top up")
	}
}

func APIUpdateUser(c *gin.Context) {
	uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
	if APIFailed(c, e, "invalid user id") {
		return
	}
	if !APIMatchSecret(c, uid) {
		return
	}

	type Input struct {
		Name string `json:"name"`

		PublicViews     bool
		PublicReadLater bool
		PublicFollowing bool
		PublicIgnored   bool
		PublicTagFollow bool

		PublicPostVotes    bool
		PublicCommentVotes bool
		PublicUserVotes    bool
		PublicTagVotes     bool
	}
	var in Input
	if e := c.BindJSON(&in); APIFailed(c, e, "get input for user permissions") {
		return
	}

	DBUpdateUserPublicPermissions(mainDB, uid,
		in.PublicViews, in.PublicReadLater, in.PublicIgnored, in.PublicFollowing,
		in.PublicPostVotes, in.PublicCommentVotes, in.PublicTagVotes, in.PublicUserVotes,
		in.PublicTagFollow)

	DBUpdateUser(mainDB, uid, in.Name)

	APIReturn(c, true, "")
}

func APIOnboard(c *gin.Context) {
	uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
	if APIFailed(c, e, "invalid user id") {
		return
	}
	type Input struct {
		Tags  []int64 `json:"tags"`
		Users []int64 `json:"users"`
	}
	var in Input
	if e := c.BindJSON(&in); APIFailed(c, e, "get input for vote post") {
		return
	}
	for _, tag := range in.Tags {
		DBCreateUserCont(mainDB, ucpTagFollow, uid, tag, -1)
	}
	for _, user := range in.Users {
		DBCreateUserCont(mainDB, ucpUserFollow, uid, user, -1)
	}

	following := DBGetUserContUsers(mainDB, true, uid, ucpUserFollow, 0, 0, "", "")
	tagFollowing := DBGetUserContTag(mainDB, true, uid, ucpTagFollow, 0, 0, "", "")
	APIReturn(c, true, gin.H{"following": following, "tags": tagFollowing})
}

func APIWatchUser(c *gin.Context) {
	uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
	if APIFailed(c, e, "invalid user id") {
		return
	}
	if !APIMatchSecret(c, uid) {
		return
	}

	pid, e := strconv.ParseInt(c.Param("pid"), 10, 64)
	if APIFailed(c, e, "invalid post id") {
		return
	}

	type Input struct {
		Time int64 `json:"time"`
	}
	var input Input
	if e := c.BindJSON(&input); APIFailed(c, e, "get input for vote post") {
		return
	}

	addr := getAddress(c.ClientIP())

	pref := DBWatchUser(mainDB, uid, pid, input.Time, addr)

	APIReturn(c, true, pref)
}

func APIOnboardAgents(c *gin.Context) {
	APIReturn(c, true, agentsOnboarding)
}

func APIOnboardTags(c *gin.Context) {
	APIReturn(c, true, tagsOnboarding)
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

	if !APIMatchSecret(c, in.UID) {
		return
	}

	DBCreateFlag(mainDB, in.UID, in.PID, in.SID, in.Kind, in.Reason)

	APIReturn(c, true, gin.H{})
}

func APIGetFlaggedPosts(c *gin.Context) {
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

	if !APIMatchSecret(c, -1) {
		return
	}

	content := DBGetFlaggedPosts(mainDB, FlagReason(kind), limit, offset)

	APIReturn(c, true, content)
}

func APIGetFlaggedComments(c *gin.Context) {
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

	if !APIMatchSecret(c, -1) {
		return
	}

	content := DBGetFlaggedComments(mainDB, FlagReason(kind), limit, offset)

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

	if !APIMatchSecret(c, -1) {
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
		DeviceId string `json:"device"`
	}
	var in Input
	if e := c.BindJSON(&in); DidFail(e, "get input for sign in") {
		APIReturn(c, false, "invalid input values")
		return
	}

	user, streak := DBSignIn(mainDB, in.Email, in.Password)
	if user.ID > 0 || user.ID == -1 {
		secret := AUTHRegister(mainDB, user.ID, in.DeviceId)
		following := DBGetUserContUsers(mainDB, true, user.ID, ucpUserFollow, 0, 0, "", "")
		ignored := DBGetUserContUsers(mainDB, true, user.ID, ucpUserIgnored, 0, 0, "", "")
		tagFollowing := DBGetUserContTag(mainDB, true, user.ID, ucpTagFollow, 0, 0, "", "")
		APIReturn(c, true, gin.H{
			"user":      user,
			"token":     secret,
			"streak":    streak,
			"following": following,
			"ignored":   ignored,
			"tags":      tagFollowing,
		})
	} else {
		if user.ID == -2 {
			APIReturn(c, false, "missing")
		} else if user.ID == -3 {
			APIReturn(c, false, "password")
		} else {
			APIReturn(c, false, "unknown")
		}
	}
}

func APIRedirectAppleSignIn(c *gin.Context) {
	type Input struct {
		Code    string `json:"code" form:"code"`
		IdToken string `json:"id_token" form:"id_token"`
	}
	var in Input
	if e := c.Bind(&in); APIFailed(c, e, "get input for redirect") {
		return
	}

	a := fmt.Sprintf("code=%s", url.QueryEscape(in.Code))
	b := fmt.Sprintf("id_token=%s", url.QueryEscape(in.IdToken))
	args := fmt.Sprintf("%s&%s", a, b)

	appleTokens.Store(in.Code, in.IdToken)
	redirect := fmt.Sprintf("intent://callback?%s#Intent;package=com.letanyan.newsource;scheme=signinwithapple;end", args)

	c.Redirect(307, redirect)
}

func APISignInWithApple(c *gin.Context) {
	type Input struct {
		Code     string `json:"code"`
		DeviceId string `json:"device"`
	}
	var in Input
	if e := c.BindJSON(&in); APIFailed(c, e, "get apple sign in token") {
		return
	}

	claims := ValidateAppleJWT(in.Code)
	if len(claims.Email) <= 0 {
		APIReturn(c, false, "validate apple sign in")
		return
	}

	user, streak := DBGetUser(mainDB, 0, claims.Email)
	if user.ID == 0 {
		newUser := DBCreateUser(mainDB, claims.FirstName, claims.Email, in.Code)
		if newUser.ID > 0 {
			secret := AUTHRegister(mainDB, newUser.ID, in.DeviceId)
			APIReturn(c, true, gin.H{
				"user":      newUser,
				"token":     secret,
				"streak":    newUser.Credits,
				"following": []UserProfile{},
				"ignored":   []UserProfile{},
				"tags":      []Tag{},
			})
		} else {
			APIReturn(c, false, "could not create user")
		}
	} else {
		secret := AUTHRegister(mainDB, user.ID, in.DeviceId)
		following := DBGetUserContUsers(mainDB, true, user.ID, ucpUserFollow, 0, 0, "", "")
		ignored := DBGetUserContUsers(mainDB, true, user.ID, ucpUserIgnored, 0, 0, "", "")
		tagFollowing := DBGetUserContTag(mainDB, true, user.ID, ucpTagFollow, 0, 0, "", "")
		DBValidateUser(mainDB, user.ID, user.ValidationKey)
		user.ValidationKey = 0
		user.Password = ""
		APIReturn(c, true, gin.H{
			"user":      user,
			"token":     secret,
			"streak":    streak,
			"following": following,
			"ignored":   ignored,
			"tags":      tagFollowing,
		})
	}
}

func APISignInWithGoogle(c *gin.Context) {
	type Input struct {
		Token    string `json:"token"`
		Access   string `json:"access"`
		DeviceId string `json:"device"`
	}
	var in Input
	if e := c.BindJSON(&in); APIFailed(c, e, "get google sign in token") {
		return
	}

	claims, e := ValidateGoogleJWT(in.Token)
	if APIFailed(c, e, "validate google JWT") {
		return
	}

	user, streak := DBGetUser(mainDB, 0, claims.Email)
	if user.ID == 0 {
		newUser := DBCreateUser(mainDB, claims.FirstName, claims.Email, in.Access)
		if newUser.ID > 0 {
			secret := AUTHRegister(mainDB, newUser.ID, in.DeviceId)
			APIReturn(c, true, gin.H{
				"user":      newUser,
				"token":     secret,
				"streak":    newUser.Credits,
				"following": []UserProfile{},
				"ignored":   []UserProfile{},
				"tags":      []Tag{},
			})
		} else {
			APIReturn(c, false, "could not create user")
		}
	} else {
		secret := AUTHRegister(mainDB, user.ID, in.DeviceId)
		following := DBGetUserContUsers(mainDB, true, user.ID, ucpUserFollow, 0, 0, "", "")
		ignored := DBGetUserContUsers(mainDB, true, user.ID, ucpUserIgnored, 0, 0, "", "")
		tagFollowing := DBGetUserContTag(mainDB, true, user.ID, ucpTagFollow, 0, 0, "", "")
		DBValidateUser(mainDB, user.ID, user.ValidationKey)
		user.ValidationKey = 0
		user.Password = ""
		APIReturn(c, true, gin.H{
			"user":      user,
			"token":     secret,
			"streak":    streak,
			"following": following,
			"ignored":   ignored,
			"tags":      tagFollowing,
		})
	}
}

func APISignOut(c *gin.Context) {
	type Input struct {
		UserId   int64  `json:"userId"`
		DeviceId string `json:"device"`
	}
	var in Input
	if e := c.BindJSON(&in); APIFailed(c, e, "get input for sign out") {
		return
	}

	AUTHDeregister(mainDB, in.UserId, in.DeviceId)

	APIReturn(c, true, "")
}

func APISendPasswordReset(c *gin.Context) {
	type Input struct {
		Email string `json:"email"`
	}

	var in Input
	if e := c.BindJSON(&in); APIFailed(c, e, "get input for password reset") {
		return
	}

	user, _ := DBGetUser(mainDB, 0, in.Email)
	if user.ID == 0 {
		APIReturn(c, false, fmt.Sprintf("no user with email '%s' exists", in.Email))
		return
	}

	rand.Seed(time.Now().Unix())
	key := rand.Int31()
	AUTHUserReset.Store(user.ID, key)
	time.AfterFunc(time.Minute*30, func() {
		AUTHUserReset.Delete(user.ID)
	})

	MailPasswordReset(user.ID, in.Email, key)
	APIReturn(c, true, 0)
}

func APIPasswordResetForm(c *gin.Context) {
	uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
	if DidFail(e, "invalid user id") {
		return
	}

	key, e := strconv.ParseInt(c.Param("key"), 10, 32)
	if DidFail(e, "invalid key") {
		return
	}

	APIReturnHTML(c, "reset_password_form.html", gin.H{"UserId": uid, "Key": key, "Src": serverAddr})
}

func APIResetPassword(c *gin.Context) {
	uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
	if DidFail(e, "invalid user id") {
		APIReturnHTML(c, "reset_password_failed.html", gin.H{})
		return
	}

	nKey, e := strconv.ParseInt(c.Param("key"), 10, 32)
	if DidFail(e, "invalid key") {
		APIReturnHTML(c, "reset_password_failed.html", gin.H{})
		return
	}

	oKey, loaded := AUTHUserReset.LoadAndDelete(uid)
	if !(loaded && int32(nKey) == oKey.(int32)) {
		APIReturnHTML(c, "reset_password_failed.html", gin.H{})
		return
	}

	type Input struct {
		Password string `json:"password"`
	}
	var in Input
	if e := c.Bind(&in); APIFailed(c, e, "get input for password reset") {
		return
	}
	DBResetPasswordForUser(mainDB, uid, password)

	APIReturnHTML(c, "reset_password_confirmed.html", gin.H{})
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

	MailValidationKey(uid, in.Email, in.Key)
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
		APIReturnHTML(c, "verify_confirmed.html", gin.H{})
	} else {
		user, _ := DBGetUser(mainDB, uid, "")
		if user.ValidationKey == 0 {
			APIReturnHTML(c, "verify_confirmed.html", gin.H{})
		} else {
			APIReturnHTML(c, "verify_failed.html", gin.H{})
		}
	}
}

func APIVerifyGoogleIAP(c *gin.Context) {
	type Input struct {
		Data      string `json:"data"`
		ProductId string `json:"productId"`
		UserId    int64  `json:"userId"`
	}
	var in Input
	if e := c.BindJSON(&in); APIFailed(c, e, "get input for google in app purchase") {
		return
	}
	if !APIMatchSecret(c, in.UserId) {
		APIReturn(c, false, -1)
		return
	}

	isAuthentic := AUTHGoogleIAP(in.Data, in.ProductId)
	if !isAuthentic {
		APIReturn(c, false, -1)
		return
	}
	amount := mapProductIdToCredit(in.ProductId)
	if amount == -1 {
		APIReturn(c, false, -1)
		return
	}
	alreadyPurchased := DBIapExists(mainDB, in.UserId, "g", in.ProductId, in.Data)
	if alreadyPurchased {
		APIReturn(c, false, -1)
		return
	}
	DBIapInsert(mainDB, in.UserId, "g", in.ProductId, in.Data)

	result := DBAddUserCredit(mainDB, in.UserId, amount)
	APIReturn(c, true, result)
}

func APIVerifyAppleIAP(c *gin.Context) {
	type Input struct {
		Data      string `json:"data"`
		ProductId string `json:"productId"`
		UserId    int64  `json:"userId"`
	}
	var in Input
	if e := c.BindJSON(&in); APIFailed(c, e, "get input for apple in app purchase") {
		return
	}
	if !APIMatchSecret(c, in.UserId) {
		APIReturn(c, false, -1)
		return
	}

	isAuthentic := AUTHAppleIAP(in.Data)
	if !isAuthentic {
		APIReturn(c, false, -1)
		return
	}
	amount := mapProductIdToCredit(in.ProductId)
	if amount == -1 {
		APIReturn(c, false, -1)
		return
	}
	alreadyPurchased := DBIapExists(mainDB, in.UserId, "a", in.ProductId, in.Data)
	if alreadyPurchased {
		APIReturn(c, false, -1)
		return
	}
	DBIapInsert(mainDB, in.UserId, "a", in.ProductId, in.Data)
	result := DBAddUserCredit(mainDB, in.UserId, amount)

	APIReturn(c, true, result)
}

func APIAvailable(c *gin.Context) {
	APIReturn(c, true, "")
}
