package main

import (
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
	uid, e := strconv.ParseInt(c.Param("uid"), 10, 64)
	if APIFailed(c, e, "invalid uid param") {
		return
	}

	result := NAUpdateNewsAgent(uid)

	APIReturn(c, true, result)
}
