package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func APIReturn(c *gin.Context, success bool, payload interface{}) {
	if isDebug {
		if success {
			c.IndentedJSON(http.StatusAccepted, gin.H{"success": true, "payload": payload})
		} else {
			c.IndentedJSON(http.StatusAccepted, gin.H{"success": false, "reason": payload})
		}
	} else {
		if success {
			c.JSON(http.StatusAccepted, gin.H{"success": true, "payload": payload})
		} else {
			c.JSON(http.StatusAccepted, gin.H{"success": false, "reason": payload})
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

	user := DBCreateUser(mainDB, input.Name, input.Email, input.Password)
	if user.ID != 0 {
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
	post := DBCreatePost(mainDB, input.UserID, input.Content, input.Tags, input.Location)
	if post.ID != 0 {
		APIReturn(c, true, post)
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

	users := DBGetUsers(mainDB, upvotes, downvotes, order, limit, offset)
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

	users := DBGetUserPrefUsers(mainDB, uid, upvoteAmount, downvoteAmount, upvotes, downvotes, order, limit, offset)
	APIReturn(c, true, users)
}

func APIGetUserPrefPosts(isComment bool) func(*gin.Context) {
	return func(c *gin.Context) {
		author, e := strconv.ParseInt(c.DefaultQuery("uid", "0"), 10, 64)
		if APIFailed(c, e, "invalid user id") {
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

		now := utc()
		lastWeek := now.AddDate(0, 0, -7)
		startDate := c.DefaultQuery("start", formatTime(lastWeek))
		endDate := c.DefaultQuery("end", formatTime(now))

		posts := DBGetUserPrefPosts(mainDB, uid, upvoteAmount, downvoteAmount, isComment, author, tags, location, upvotes, downvotes, order, limit, offset, startDate, endDate)
		APIReturn(c, true, posts)
	}
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

	result := DBGetUserPrefTags(mainDB, uid, upvoteAmount, downvoteAmount, tags, location, upvotes, downvotes, order, limit, offset)
	APIReturn(c, true, result)
}

func APIGetUserContent(isPost bool) func(*gin.Context) {
	return func(c *gin.Context) {
		uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
		if APIFailed(c, e, "invalid uid given") {
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

		now := utc()
		lastWeek := now.AddDate(0, 0, -7)
		startDate := c.DefaultQuery("start", formatTime(lastWeek))
		endDate := c.DefaultQuery("end", formatTime(now))

		result := DBGetUserCont(mainDB, uid, isPost, tags, location, upvotes, downvotes, order, limit, offset, startDate, endDate)
		APIReturn(c, true, result)
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

	now := utc()
	lastWeek := now.AddDate(0, 0, -7)
	startDate := c.DefaultQuery("start", formatTime(lastWeek))
	endDate := c.DefaultQuery("end", formatTime(now))

	posts := DBGetPosts(mainDB, uid, tags, location, upvotes, downvotes, order, limit, offset, startDate, endDate)
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

	pid, e := strconv.ParseInt(c.Param("pid"), 10, 64)
	if APIFailed(c, e, "invalid post id") {
		return
	}

	replyId, e := strconv.ParseInt(c.DefaultQuery("reply", "0"), 10, 64)
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

	startDate := c.DefaultQuery("start", "")
	endDate := c.DefaultQuery("end", "")

	result := DBGetComments(mainDB, pid, uid, replyId, startDate, endDate, upvotes, downvotes, order, limit, offset)
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

	tag := DBGetTags(mainDB, tid, []string{}, []string{}, 0, 0, soScore, 1, 0)
	if len(tag) == 1 {
		APIReturn(c, true, tag[0])
	} else {
		APIReturn(c, false, "no tag found with id "+fmt.Sprint(tid))
	}
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

	result := DBGetTags(mainDB, 0, tags, location, upvotes, downvotes, order, limit, offset)
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

	profile, pref := DBVoteForUser(mainDB, input.UID, targetId, input.Amount)
	remaining := DBSubtractUserCredit(mainDB, input.UID, input.Amount)

	if remaining >= 0 {
		APIReturn(c, true, gin.H{"profile": profile, "pref": pref, "credits_remaining": remaining})
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

	post, profile, tag, pref := DBVotePost(mainDB, input.UID, pid, input.Amount, addr)
	remaining := DBSubtractUserCredit(mainDB, input.UID, input.Amount)

	if remaining >= 0 {
		APIReturn(c, true, gin.H{"post": post, "profile": profile, "tag": tag, "pref": pref, "credits_remaining": remaining})
	} else {
		APIFailed(c, errors.New(""), "could not complete vote")
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

	comment, profile, pref := DBVoteComment(mainDB, input.UID, pid, cid, input.Amount)
	remaining := DBSubtractUserCredit(mainDB, input.UID, input.Amount)

	if remaining >= 0 {
		APIReturn(c, true, gin.H{"comment": comment, "profile": profile, "pref": pref, "credits_remaining": remaining})
	} else {
		APIFailed(c, errors.New(""), "could not complete vote")
	}
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
