package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/franchizzle/task-manager/backend/config"
	"github.com/franchizzle/task-manager/backend/database"
	"github.com/franchizzle/task-manager/backend/external"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
)

func TestLoginRedirect(t *testing.T) {
	api, dbCleanup := GetAPIWithDBCleanup()
	defer dbCleanup()
	api.ExternalConfig.GoogleLoginConfig = &external.OauthConfig{Config: &oauth2.Config{
		ClientID:    "123",
		RedirectURL: "g.com",
		Scopes:      []string{"s1", "s2"},
	}}
	router := GetRouter(api)
	// Syntax taken from https://semaphoreci.com/community/tutorials/test-driven-development-of-go-web-applications-with-gin
	// Also inspired by https://dev.to/jacobsngoodwin/04-testing-first-gin-http-handler-9m0
	t.Run("Success", func(t *testing.T) {
		request, _ := http.NewRequest("GET", "/login/", nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusFound, recorder.Code)

		var stateToken string
		for _, c := range recorder.Result().Cookies() {
			if c.Name == "loginStateToken" {
				stateToken = c.Value
			}
		}

		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Equal(
			t,
			"<a href=\"/login/?access_type=offline&amp;client_id=123&amp;include_granted_scopes=false&amp;redirect_uri=g.com&amp;response_type=code&amp;scope=s1+s2&amp;state="+stateToken+"\">Found</a>.\n\n",
			string(body),
		)

		stateTokenID, err := primitive.ObjectIDFromHex(stateToken)
		assert.NoError(t, err)
		token, err := database.GetStateToken(api.DB, stateTokenID, nil)
		assert.NoError(t, err)
		assert.False(t, token.UseDeeplink)
	})
	t.Run("SuccessForce", func(t *testing.T) {
		request, _ := http.NewRequest("GET", "/login/?force_prompt=true", nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusFound, recorder.Code)

		var stateToken string
		for _, c := range recorder.Result().Cookies() {
			if c.Name == "loginStateToken" {
				stateToken = c.Value
			}
		}

		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Equal(
			t,
			"<a href=\"/login/?access_type=offline&amp;client_id=123&amp;include_granted_scopes=false&amp;prompt=consent&amp;redirect_uri=g.com&amp;response_type=code&amp;scope=s1+s2&amp;state="+stateToken+"\">Found</a>.\n\n",
			string(body),
		)

		stateTokenID, err := primitive.ObjectIDFromHex(stateToken)
		assert.NoError(t, err)
		token, err := database.GetStateToken(api.DB, stateTokenID, nil)
		assert.NoError(t, err)
		assert.False(t, token.UseDeeplink)
	})
	t.Run("SuccessDeeplink", func(t *testing.T) {
		request, _ := http.NewRequest("GET", "/login/?use_deeplink=true", nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusFound, recorder.Code)

		var stateToken string
		for _, c := range recorder.Result().Cookies() {
			if c.Name == "loginStateToken" {
				stateToken = c.Value
			}
		}

		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Equal(
			t,
			"<a href=\"/login/?access_type=offline&amp;client_id=123&amp;include_granted_scopes=false&amp;redirect_uri=g.com&amp;response_type=code&amp;scope=s1+s2&amp;state="+stateToken+"\">Found</a>.\n\n",
			string(body),
		)

		stateTokenID, err := primitive.ObjectIDFromHex(stateToken)
		assert.NoError(t, err)
		token, err := database.GetStateToken(api.DB, stateTokenID, nil)
		assert.NoError(t, err)
		assert.True(t, token.UseDeeplink)
	})
}

func TestLoginCallback(t *testing.T) {
	api, dbCleanup := GetAPIWithDBCleanup()
	defer dbCleanup()
	waitlistCollection := database.GetWaitlistCollection(api.DB)

	t.Run("MissingQueryParams", func(t *testing.T) {
		router := GetRouter(api)
		request, _ := http.NewRequest("GET", "/login/callback/", nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusFound, recorder.Code)
		// check that we redirect to the home page
		assert.Equal(t, config.GetConfigValue("HOME_URL"), recorder.Result().Header.Get("Location"))
	})

	t.Run("Idempotent", func(t *testing.T) {
		recorder := makeLoginCallbackRequest("noice420", "approved@resonant-kelpie-404a42.netlify.app", "", "example-token", "example-token", true, false)
		assert.Equal(t, http.StatusFound, recorder.Code)
		verifyLoginCallback(t, api.DB, "approved@resonant-kelpie-404a42.netlify.app", "noice420", false, true)
		//change token and verify token updates and still only 1 row per user.
		recorder = makeLoginCallbackRequest("TSLA", "approved@resonant-kelpie-404a42.netlify.app", "", "example-token", "example-token", true, false)
		assert.Equal(t, http.StatusFound, recorder.Code)
		verifyLoginCallback(t, api.DB, "approved@resonant-kelpie-404a42.netlify.app", "TSLA", false, true)
	})
	t.Run("UpdatesName", func(t *testing.T) {
		userCollection := database.GetUserCollection(api.DB)
		recorder := makeLoginCallbackRequest("noice420", "approved@resonant-kelpie-404a42.netlify.app", "Task Destroyer", "example-token", "example-token", true, false)
		assert.Equal(t, http.StatusFound, recorder.Code)
		var userObject database.User
		userCollection.FindOne(context.Background(), bson.M{"google_id": "goog12345_approved@resonant-kelpie-404a42.netlify.app"}).Decode(&userObject)
		assert.Equal(t, "Task Destroyer", userObject.Name)

		recorder = makeLoginCallbackRequest("noice420", "approved@resonant-kelpie-404a42.netlify.app", "Elon Musk", "example-token", "example-token", true, false)
		assert.Equal(t, http.StatusFound, recorder.Code)
		userCollection.FindOne(context.Background(), bson.M{"google_id": "goog12345_approved@resonant-kelpie-404a42.netlify.app"}).Decode(&userObject)
		assert.Equal(t, "Elon Musk", userObject.Name)
	})
	t.Run("BadStateTokenFormat", func(t *testing.T) {
		recorder := makeLoginCallbackRequest("noice420", "approved@resonant-kelpie-404a42.netlify.app", "", "example-token", "example-token", false, false)
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Equal(t, "{\"detail\":\"invalid state token format\"}", string(body))
	})
	t.Run("BadStateTokenCookieFormat", func(t *testing.T) {
		recorder := makeLoginCallbackRequest("noice420", "approved@resonant-kelpie-404a42.netlify.app", "", "6088e1c97018a22f240aa573", "example-token", false, false)
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Equal(t, "{\"detail\":\"invalid state token cookie format\"}", string(body))
	})
	t.Run("StateTokensDontMatch", func(t *testing.T) {
		recorder := makeLoginCallbackRequest("noice420", "approved@resonant-kelpie-404a42.netlify.app", "", "6088e1c97018a22f240aa573", "6088e1c97018a22f240aa574", false, false)
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Equal(t, "{\"detail\":\"state token does not match cookie\"}", string(body))
	})
	t.Run("InvalidStateToken", func(t *testing.T) {
		recorder := makeLoginCallbackRequest("noice420", "approved@resonant-kelpie-404a42.netlify.app", "", "6088e1c97018a22f240aa573", "6088e1c97018a22f240aa573", false, false)
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Equal(t, "{\"detail\":\"invalid state token\"}", string(body))
	})
	t.Run("SuccessSecondTime", func(t *testing.T) {
		// Verifies request succeeds on second auth (no refresh token supplied)
		_, err := database.GetExternalTokenCollection(api.DB).DeleteOne(context.Background(), bson.M{"$and": []bson.M{{"account_id": "approved@resonant-kelpie-404a42.netlify.app"}, {"service_id": external.TASK_SERVICE_ID_GOOGLE}}})
		assert.NoError(t, err)
		stateToken, err := newStateToken(api.DB, "", false)
		assert.NoError(t, err)
		recorder := makeLoginCallbackRequest("noice420", "approved@resonant-kelpie-404a42.netlify.app", "", *stateToken, *stateToken, false, true)
		assert.Equal(t, http.StatusFound, recorder.Code)
		verifyLoginCallback(t, api.DB, "approved@resonant-kelpie-404a42.netlify.app", "noice420", true, true)
	})
	t.Run("Success", func(t *testing.T) {
		stateToken, err := newStateToken(api.DB, "", false)
		assert.NoError(t, err)
		recorder := makeLoginCallbackRequest("noice420", "approved@resonant-kelpie-404a42.netlify.app", "", *stateToken, *stateToken, false, false)
		assert.Equal(t, http.StatusFound, recorder.Code)
		verifyLoginCallback(t, api.DB, "approved@resonant-kelpie-404a42.netlify.app", "noice420", false, true)
	})
	t.Run("CorrectRedirectNewAccount", func(t *testing.T) {
		stateToken, err := newStateToken(api.DB, "", false)
		assert.NoError(t, err)
		recorder := makeLoginCallbackRequest("noice420", "newAccount@resonant-kelpie-404a42.netlify.app", "", *stateToken, *stateToken, false, true)
		assert.Equal(t, http.StatusFound, recorder.Code)
		assert.Equal(t, "http://localhost:3000/tos-summary", recorder.Header().Get("Location"))
	})
	t.Run("CorrectRedirectExistingAccount", func(t *testing.T) {
		userCollection := database.GetUserCollection(api.DB)
		userCollection.InsertOne(context.Background(), database.User{
			GoogleID: "goog12345_existingAccount@resonant-kelpie-404a42.netlify.app",
		})
		_, err := database.GetExternalTokenCollection(api.DB).DeleteOne(context.Background(), bson.M{"$and": []bson.M{{"account_id": "approved@resonant-kelpie-404a42.netlify.app"}, {"service_id": external.TASK_SERVICE_ID_GOOGLE}}})
		assert.NoError(t, err)
		stateToken, err := newStateToken(api.DB, "", false)
		assert.NoError(t, err)
		recorder := makeLoginCallbackRequest("noice420", "existingAccount@resonant-kelpie-404a42.netlify.app", "", *stateToken, *stateToken, false, true)
		assert.Equal(t, http.StatusFound, recorder.Code)
		assert.Equal(t, "http://localhost:3000/", recorder.Header().Get("Location"))
	})
	t.Run("SuccessDeeplink", func(t *testing.T) {
		stateToken, err := newStateToken(api.DB, "", true)
		assert.NoError(t, err)
		recorder := makeLoginCallbackRequest("noice420", "approved@resonant-kelpie-404a42.netlify.app", "", *stateToken, *stateToken, false, false)
		assert.Equal(t, http.StatusFound, recorder.Code)
		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Contains(t, string(body), "generaltask://authentication?authToken=")
		verifyLoginCallback(t, api.DB, "approved@resonant-kelpie-404a42.netlify.app", "noice420", false, true)
	})
	t.Run("SuccessWaitlist", func(t *testing.T) {
		_, err := waitlistCollection.InsertOne(
			context.Background(),
			&database.WaitlistEntry{
				Email:     "dogecoin@tothe.moon",
				HasAccess: true,
			},
		)
		assert.NoError(t, err)
		stateToken, err := newStateToken(api.DB, "", false)
		assert.NoError(t, err)
		recorder := makeLoginCallbackRequest("noice420", "dogecoin@tothe.moon", "", *stateToken, *stateToken, false, false)
		assert.Equal(t, http.StatusFound, recorder.Code)
		verifyLoginCallback(t, api.DB, "dogecoin@tothe.moon", "noice420", false, true)
	})
}
