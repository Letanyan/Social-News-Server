package main

import (
	"github.com/gin-gonic/gin"
)

func ServeAPI(router *gin.Engine) {
	api := router.Group("/api")
	v1 := api.Group("/v1")
	{
		// AUTH
		v1.POST("/auth/sign-in", APISignIn)
		v1.POST("/auth/callbacks/sign-in-with-apple", APIRedirectAppleSignIn)
		v1.POST("/auth/sign-in-with-apple", APISignInWithApple)
		v1.POST("/auth/sign-in-with-google", APISignInWithGoogle)
		v1.POST("/auth/sign-out", APISignOut)
		v1.POST("/auth/verification/users/:uid", APIResendVerificationLink)
		v1.GET("/users/:uid/verification/:key", APIVerifyUserEmail)

		v1.POST("/auth/password-reset", APISendPasswordReset)
		v1.GET("/users/:uid/password-reset/:key", APIPasswordResetForm)
		v1.POST("/users/:uid/password-reset/:key", APIResetPassword)

		v1.POST("/auth/google-iap", APIVerifyGoogleIAP)
		v1.POST("/auth/apple-iap", APIVerifyAppleIAP)
		v1.GET("/available", APIAvailable)
		// Create
		v1.POST("/users", APICreateUser)
		v1.POST("/posts", APICreatePost)
		v1.POST("/posts/:pid/comments", APICreateComment)
		v1.POST("/update/posts/:pid", APIUpdatePost)
		v1.POST("/update/posts/:pid/comments/:cid", APIUpdateComment)
		// Delete
		v1.POST("/trash/users/:uid", APIDeleteUser)
		v1.POST("/trash/posts/:pid", APIDeletePost)
		v1.POST("/trash/posts/:pid/comments/:cid", APIDeleteComment)

		v1.POST("/trash/users/:uid/content/posts/read-later/:pid", APIDeleteUserContPlaylist(ucpReadLater))
		v1.POST("/trash/users/:uid/content/posts/viewed/:pid", APIDeleteUserContPlaylist(ucpViewed))
		v1.POST("/trash/users/:uid/content/user-follows/:pid", APIDeleteUserContPlaylist(ucpUserFollow))
		v1.POST("/trash/users/:uid/content/ignored/:pid", APIDeleteUserContPlaylist(ucpUserIgnored))
		v1.POST("/trash/users/:uid/content/tag-follows/:pid", APIDeleteUserContPlaylist(ucpTagFollow))

		// Get
		v1.GET("/users/:uid", APIGetUser)

		v1.GET("/users/:uid/prefs/users", APIGetUserPrefUsers)
		v1.GET("/users/:uid/prefs/posts", APIGetUserPrefPosts)
		v1.GET("/users/:uid/prefs/comments", APIGetUserPrefComments)
		v1.GET("/users/:uid/prefs/tags", APIGetUserPrefTags)

		v1.GET("/users/:uid/content/posts", APIGetUserContPost(ucpCreated))
		v1.GET("/users/:uid/content/posts/read-later", APIGetUserContPost(ucpReadLater))
		v1.GET("/users/:uid/content/posts/viewed", APIGetUserContPost(ucpViewed))
		v1.GET("/users/:uid/content/comments", APIGetUserContComments)
		v1.GET("/users/:uid/content/user-follows", APIGetUserContUsers(ucpUserFollow))
		v1.GET("/users/:uid/content/ignored", APIGetUserContUsers(ucpUserIgnored))
		v1.GET("/users/:uid/content/tag-follows", APIGetUserContTags(ucpTagFollow))

		v1.GET("/users/:uid/recommend", APIGetSimilarPosts)

		v1.GET("/users/prefs/users", APIGetUserPrefsFor(upUser))
		v1.GET("/users/prefs/posts", APIGetUserPrefsFor(upPost))
		v1.GET("/users/prefs/comments", APIGetUserPrefsFor(upComment))
		v1.GET("/users/prefs/tags", APIGetUserPrefsFor(upTag))

		v1.GET("/users/content/read-later", APIGetUserContsFor(ucpReadLater))
		v1.GET("/users/content/viewed", APIGetUserContsFor(ucpViewed))
		v1.GET("/users/content/user-follows", APIGetUserContsFor(ucpUserFollow))
		v1.GET("/users/content/ignored", APIGetUserContsFor(ucpUserIgnored))
		v1.GET("/users/content/tag-follows", APIGetUserContsFor(ucpTagFollow))

		v1.GET("/users", APIGetUsers)
		v1.GET("/posts/:pid", APIGetPost)
		v1.GET("/posts", APIGetPosts)
		v1.GET("/posts/:pid/comments/:cid", APIGetComment)
		v1.GET("/posts/comments", APIGetComments)
		v1.GET("/tags/:tid", APIGetTag)
		v1.GET("/tags", APIGetTags)
		v1.POST("/tags", APIGetTagsFromIDs)
		// Vote
		v1.POST("/credits/:uid", APIPurchaseCredit)
		v1.POST("/users/:uid/details", APIUpdateUser)
		v1.POST("/users/:uid", APIVoteUser)
		v1.POST("/posts/:pid", APIVotePost)
		v1.POST("/posts/:pid/comments/:cid", APIVoteComment)

		v1.POST("/users/:uid/content/posts/read-later", APIAddUserCont(ucpReadLater))
		v1.POST("/users/:uid/content/posts/viewed", APIAddUserCont(ucpViewed))
		v1.POST("/users/:uid/content/user-follows", APIAddUserCont(ucpUserFollow))
		v1.POST("/users/:uid/content/ignored", APIAddUserCont(ucpUserIgnored))
		v1.POST("/users/:uid/content/tag-follows", APIAddUserCont(ucpTagFollow))
		v1.POST("/users/:uid/content/recommendations", APIRefreshUserContRecommendations)

		v1.POST("/users/:uid/content/onboard", APIOnboard)
		v1.POST("/users/:uid/watch/:pid", APIWatchUser)
		v1.GET("/onboard/agents", APIOnboardAgents)
		v1.GET("/onboard/tags", APIOnboardTags)

		//Flags
		v1.POST("/flags", APICreateFlag)
		v1.GET("/flags/posts", APIGetFlaggedPosts)
		v1.GET("/flags/comments", APIGetFlaggedComments)
		v1.POST("/trash/flags/:id", APIHandleFlag)

		// Agents
		v1.POST("/agents", APICreateAgent)
		v1.POST("/agents/:aid", APIUpdateAgent)
		v1.POST("/trash/agents/:aid", APIDeleteAgent)
		v1.POST("/all-agents", APIUpdateAgents)
		v1.GET("/agents/:aid", APIGetAgent)
		v1.GET("/agents", APIGetAllAgents)
		v1.POST("/update/agents/:aid", APIEditAgent)
		v1.POST("/agents/:aid/sub", APIEditSubNewsAgent)
	}
}
