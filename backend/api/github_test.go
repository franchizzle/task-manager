package api

import (
	"net/http"
	"testing"

	"github.com/franchizzle/task-manager/backend/config"
	"github.com/franchizzle/task-manager/backend/external"
	"github.com/franchizzle/task-manager/backend/testutils"
)

const DefaultTokenPayload string = `{"access_token":"sample-access-token", "data": {"email": "moon@dogecoin.tesla"}}`

func TestLinkGithub(t *testing.T) {
	api, cleanup := GetAPIWithDBCleanup()
	defer cleanup()
	t.Run("CookieMissing", func(t *testing.T) {
		TestAuthorizeCookieMissing(t, api, "/link/github/")
	})
	t.Run("CookieBad", func(t *testing.T) {
		TestAuthorizeCookieBad(t, api, "/link/github/")
	})
	t.Run("Success", func(t *testing.T) {
		TestAuthorizeSuccess(t, api, "/link/github/", func(stateToken string) string {
			return "<a href=\"https://github.com/login/oauth/authorize?access_type=offline&amp;client_id=" + config.GetConfigValue("GITHUB_OAUTH_CLIENT_ID") + "&amp;prompt=consent&amp;redirect_uri=http%3A%2F%2Flocalhost%3A8080%2Flink%2Fgithub%2Fcallback%2F&amp;response_type=code&amp;scope=repo&amp;state=" + stateToken + "\">Found</a>.\n\n"
		})
	})
}

func TestLinkGithubCallback(t *testing.T) {
	api, cleanup := GetAPIWithDBCleanup()
	defer cleanup()

	t.Run("CookieMissing", func(t *testing.T) {
		TestAuthorizeCookieMissing(t, api, "/link/github/callback/")
	})
	t.Run("CookieBad", func(t *testing.T) {
		TestAuthorizeCookieBad(t, api, "/link/github/callback/")
	})
	t.Run("MissingCodeParam", func(t *testing.T) {
		TestAuthorizeCallbackMissingCodeParam(t, api, "/link/github/callback/")
	})
	t.Run("BadStateTokenFormat", func(t *testing.T) {
		TestAuthorizeCallbackMissingCodeParam(t, api, "/link/github/callback/")
	})
	t.Run("InvalidStateToken", func(t *testing.T) {
		TestAuthorizeCallbackInvalidStateToken(t, api, "/link/github/callback/")
	})
	t.Run("InvalidStateTokenWrongUser", func(t *testing.T) {
		TestAuthorizeCallbackInvalidStateToken(t, api, "/link/github/callback/")
	})
	t.Run("UnsuccessfulResponse", func(t *testing.T) {
		server := testutils.GetMockAPIServer(t, http.StatusUnauthorized, DefaultTokenPayload)
		(api.ExternalConfig.Github.OauthConfig.(*external.OauthConfig)).Config.Endpoint.TokenURL = server.URL
		TestAuthorizeCallbackUnsuccessfulResponse(t, api, "/link/github/callback/")
	})
	t.Run("Success", func(t *testing.T) {
		server := testutils.GetMockAPIServer(t, http.StatusOK, DefaultTokenPayload)
		(api.ExternalConfig.Github.OauthConfig.(*external.OauthConfig)).Config.Endpoint.TokenURL = server.URL

		accountIdServer := testutils.GetMockAPIServer(t, http.StatusOK, testutils.UserResponsePayload)
		api.ExternalConfig.Github.ConfigValues.GetUserURL = &accountIdServer.URL
		TestAuthorizeCallbackSuccessfulResponse(t, api, "/link/github/callback/", external.TASK_SERVICE_ID_GITHUB)
	})
}
