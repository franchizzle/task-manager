package api

import (
	"github.com/GeneralTask/task-manager/backend/config"
	"github.com/GeneralTask/task-manager/backend/external"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/oauth2"
)

type SlackRedirectParams struct {
	Code  string `form:"code" binding:"required"`
	State string `form:"state" binding:"required"`
}

func GetSlackConfig() *external.OauthConfig {
	return &external.OauthConfig{Config: &oauth2.Config{
		ClientID:     config.GetConfigValue("SLACK_OAUTH_CLIENT_ID"),
		ClientSecret: config.GetConfigValue("SLACK_OAUTH_CLIENT_SECRET"),
		RedirectURL:  "https://api.generaltask.io/authorize/slack/callback",
		Scopes:       []string{"channels:history", "channels:read", "im:read", "mpim:history", "im:history", "groups:history", "groups:read", "mpim:write", "im:write", "channels:write", "groups:write", "chat:write:user"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://slack.com/oauth/authorize",
			TokenURL: "https://slack.com/api/oauth.access",
		},
	}}
}

func (api *API) AuthorizeSlack(c *gin.Context) {
	internalToken, err := getTokenFromCookie(c)
	if err != nil {
		return
	}

	slack := external.SlackService{Config: api.SlackConfig}
	authURL, err := slack.GetLinkAuthURL(internalToken.UserID)
	c.Redirect(302, *authURL)
}

func (api *API) AuthorizeSlackCallback(c *gin.Context) {
	internalToken, err := getTokenFromCookie(c)
	if err != nil {
		return
	}

	var redirectParams SlackRedirectParams
	if c.ShouldBind(&redirectParams) != nil {
		c.JSON(400, gin.H{"detail": "Missing query params"})
		return
	}

	stateTokenID, err := primitive.ObjectIDFromHex(redirectParams.State)
	if err != nil {
		c.JSON(400, gin.H{"detail": "invalid state token format"})
		return
	}

	slack := external.SlackService{Config: api.SlackConfig}
	err = slack.HandleAuthCallback(redirectParams.Code, stateTokenID, internalToken.UserID)
	if err != nil {
		c.JSON(500, gin.H{"detail": err.Error()})
		return
	}
	c.Redirect(302, config.GetConfigValue("HOME_URL"))
}
