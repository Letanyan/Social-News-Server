package main

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
)

func APICreateAgent(c *gin.Context) {
	type Input struct {
		Name   string `json:"name"`
		Origin string `json:"origin"`
		Locale string `json:"locale"`
	}
	var in Input

	if e := c.BindJSON(&in); DidFail(e, "get input for create user") {
		APIReturn(c, false, "invalid input values")
		return
	}

	if !APIMatchSecret(c, -1) {
		return
	}

	user := DBCreateUser(mainDB, in.Name, "", "")
	DBValidateUser(mainDB, user.ID, user.ValidationKey)
	if user.ID > 0 {
		agent, e := NACreateNewsAgent(user.ID, in.Name, in.Origin, in.Locale)
		if e == nil {
			APIReturn(c, true, agent)
		} else {
			APIReturn(c, false, "could not create agent: "+e.Error())
		}
	} else {
		APIReturn(c, false, "could not create agent")
	}
}

func APIEditAgent(c *gin.Context) {
	type Input struct {
		Name   string `json:"name"`
		Origin string `json:"origin"`
	}
	var in Input

	if e := c.BindJSON(&in); DidFail(e, "get input for create user") {
		APIReturn(c, false, "invalid input values")
		return
	}

	if !APIMatchSecret(c, -1) {
		return
	}

	aid, e := strconv.ParseInt(c.Param("aid"), 10, 64)
	if APIFailed(c, e, "invalid aid param") {
		return
	}

	agent := NAEditNewsAgent(mainDB, aid, in.Name, in.Origin)

	APIReturn(c, true, agent)
}

func APIEditSubNewsAgent(c *gin.Context) {
	type Input struct {
		Sub string `json:"sub"`
	}
	var in Input

	if e := c.BindJSON(&in); DidFail(e, "get input for news agent sub") {
		APIReturn(c, false, "invalid input values")
		return
	}

	if !APIMatchSecret(c, -1) {
		return
	}

	aid, e := strconv.ParseInt(c.Param("aid"), 10, 64)
	if APIFailed(c, e, "invalid aid param") {
		return
	}

	var agent NewsAgent
	if in.Sub[0] == '+' {
		agent = NAAddSub(aid, in.Sub[1:len(in.Sub)])
	} else if in.Sub[0] == '-' {
		agent = NARemoveSub(aid, in.Sub[1:len(in.Sub)])
	}
	APIReturn(c, true, agent)
}

func APIDeleteAgent(c *gin.Context) {
	aid, e := strconv.ParseInt(c.Param("aid"), 10, 64)
	if APIFailed(c, e, "invalid aid param") {
		return
	}

	if !APIMatchSecret(c, -1) {
		return
	}

	e = NADeleteNewsAgent(aid)
	if APIFailed(c, e, "no agent exists") {
		return
	}

	DBDeleteUser(mainDB, aid)

	APIReturn(c, true, "deleted agent")
}

func APIUpdateAgent(c *gin.Context) {
	aid, e := strconv.ParseInt(c.Param("aid"), 10, 64)
	if APIFailed(c, e, "invalid aid param") {
		return
	}

	if !APIMatchSecret(c, -1) {
		return
	}

	result := NAUpdateNewsAgentWithID(mainDB, aid)

	APIReturn(c, true, result)
}

func APIUpdateAgents(c *gin.Context) {
	if !APIMatchSecret(c, -1) {
		return
	}

	result := NAUpdateAllNewsAgent(mainDB, 0)

	APIReturn(c, true, result)
}

func APIGetAgent(c *gin.Context) {
	aid, e := strconv.ParseInt(c.Param("aid"), 10, 64)
	if APIFailed(c, e, "invalid aid param") {
		return
	}
	if !APIMatchSecret(c, -1) {
		return
	}

	var result NewsAgent
	for _, a := range agents {
		if a.ID == aid {
			result = a
			break
		}
	}

	if result.ID > 0 {
		APIReturn(c, true, result)
	} else {
		APIFailed(c, errors.New("could not find agent with id"), "could not find agent with id")
	}
}

func APIGetAllAgents(c *gin.Context) {
	if !APIMatchSecret(c, -1) {
		return
	}
	APIReturn(c, true, agents)
}
