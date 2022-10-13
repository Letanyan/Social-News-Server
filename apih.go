package main

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
)

func APIHCreateAgent(c *gin.Context) {
	type Input struct {
		Name   string `json:"name"`
		Origin string `json:"origin"`
	}
	var input Input

	if e := c.BindJSON(&input); DidFail(e, "get input for create user") {
		APIReturn(c, false, "invalid input values")
		return
	}

	user := DBCreateUser(mainDB, input.Name, "", "")
	DBValidateUser(mainDB, user.ID, user.ValidationKey)
	if user.ID != 0 {
		agent, e := NACreateNewsAgent(user.ID, input.Name, input.Origin)
		if e == nil {
			APIReturn(c, true, agent)
		} else {
			APIReturn(c, false, "could not create agent: "+e.Error())
		}
	} else {
		APIReturn(c, false, "could not create agent")
	}
}

func APIHUpdateAgent(c *gin.Context) {
	aid, e := strconv.ParseInt(c.Param("aid"), 10, 64)
	if APIFailed(c, e, "invalid aid param") {
		return
	}

	result := NAUpdateNewsAgentWithID(aid)

	APIReturn(c, true, result)
}

func APIHUpdateAgents(c *gin.Context) {
	result := NAUpdateAllNewsAgent()

	APIReturn(c, true, result)
}

func APIHGetAgent(c *gin.Context) {
	aid, e := strconv.ParseInt(c.Param("aid"), 10, 64)
	if APIFailed(c, e, "invalid aid param") {
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

func APIHGetAllAgents(c *gin.Context) {
	APIReturn(c, true, agents)
}
