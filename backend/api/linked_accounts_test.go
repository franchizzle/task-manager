package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/franchizzle/task-manager/backend/database"
	"github.com/franchizzle/task-manager/backend/external"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestSupportedAccountTypesList(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		authToken := login("approved@resonant-kelpie-404a42.netlify.app", "")
		api, dbCleanup := GetAPIWithDBCleanup()
		defer dbCleanup()
		router := GetRouter(api)
		request, _ := http.NewRequest("GET", "/linked_accounts/supported_types/", nil)
		request.Header.Add("Authorization", "Bearer "+authToken)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusOK, recorder.Code)
		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.True(t, strings.Contains(string(body), "{\"name\":\"Google Calendar\",\"logo\":\"/images/gcal.png\",\"logo_v2\":\"gcal\",\"authorization_url\":\"http://localhost:8080/link/google/\"}"))
		assert.Equal(t, 1, strings.Count(string(body), "{\"name\":\"Slack\",\"logo\":\"/images/slack.svg\",\"logo_v2\":\"slack\",\"authorization_url\":\"http://localhost:8080/link/slack/\"}"))
		assert.Equal(t, 1, strings.Count(string(body), "{\"name\":\"Jira\",\"logo\":\"/images/jira.svg\",\"logo_v2\":\"jira\",\"authorization_url\":\"http://localhost:8080/link/atlassian/\"}"))
	})
	UnauthorizedTest(t, "GET", "/linked_accounts/supported_types/", nil)
}

func TestLinkedAccountsList(t *testing.T) {
	api, dbCleanup := GetAPIWithDBCleanup()
	defer dbCleanup()
	t.Run("SuccessOnlyGoogle", func(t *testing.T) {
		authToken := login("linkedaccounts@resonant-kelpie-404a42.netlify.app", "")
		router := GetRouter(api)
		request, _ := http.NewRequest("GET", "/linked_accounts/", nil)
		request.Header.Add("Authorization", "Bearer "+authToken)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusOK, recorder.Code)
		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		googleTokenID := getGoogleTokenFromAuthToken(t, api.DB, authToken).ID.Hex()
		assert.Equal(t, "[{\"id\":\""+googleTokenID+"\",\"display_id\":\"linkedaccounts@resonant-kelpie-404a42.netlify.app\",\"name\":\"Google Calendar\",\"logo\":\"/images/gcal.png\",\"logo_v2\":\"gcal\",\"is_unlinkable\":false,\"has_bad_token\":false}]", string(body))
	})
	t.Run("Success", func(t *testing.T) {
		authToken := login("linkedaccounts2@resonant-kelpie-404a42.netlify.app", "")
		linearTokenID := insertLinearToken(t, api.DB, authToken, false).Hex()
		router := GetRouter(api)
		request, _ := http.NewRequest("GET", "/linked_accounts/", nil)
		request.Header.Add("Authorization", "Bearer "+authToken)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusOK, recorder.Code)
		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		googleTokenID := getGoogleTokenFromAuthToken(t, api.DB, authToken).ID.Hex()
		assert.Equal(t, "[{\"id\":\""+googleTokenID+"\",\"display_id\":\"linkedaccounts2@resonant-kelpie-404a42.netlify.app\",\"name\":\"Google Calendar\",\"logo\":\"/images/gcal.png\",\"logo_v2\":\"gcal\",\"is_unlinkable\":false,\"has_bad_token\":false},{\"id\":\""+linearTokenID+"\",\"display_id\":\"Linear\",\"name\":\"Linear\",\"logo\":\"/images/linear.png\",\"logo_v2\":\"linear\",\"is_unlinkable\":true,\"has_bad_token\":false}]", string(body))

	})

	t.Run("SuccessWithBadToken", func(t *testing.T) {
		authToken := login("linkedaccounts3@resonant-kelpie-404a42.netlify.app", "")
		linearTokenID := insertLinearToken(t, api.DB, authToken, true).Hex()

		api, dbCleanup := GetAPIWithDBCleanup()
		defer dbCleanup()
		router := GetRouter(api)
		request, _ := http.NewRequest("GET", "/linked_accounts/", nil)
		request.Header.Add("Authorization", "Bearer "+authToken)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusOK, recorder.Code)
		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		googleTokenID := getGoogleTokenFromAuthToken(t, api.DB, authToken).ID.Hex()
		assert.Equal(t, "[{\"id\":\""+googleTokenID+"\",\"display_id\":\"linkedaccounts3@resonant-kelpie-404a42.netlify.app\",\"name\":\"Google Calendar\",\"logo\":\"/images/gcal.png\",\"logo_v2\":\"gcal\",\"is_unlinkable\":false,\"has_bad_token\":false},{\"id\":\""+linearTokenID+"\",\"display_id\":\"Linear\",\"name\":\"Linear\",\"logo\":\"/images/linear.png\",\"logo_v2\":\"linear\",\"is_unlinkable\":true,\"has_bad_token\":true}]", string(body))
	})
	UnauthorizedTest(t, "GET", "/linked_accounts/", nil)
}

func TestDeleteLinkedAccount(t *testing.T) {
	api, dbCleanup := GetAPIWithDBCleanup()
	defer dbCleanup()
	t.Run("MalformattedAccountID", func(t *testing.T) {
		authToken := login("approved@resonant-kelpie-404a42.netlify.app", "")
		ServeRequest(t, authToken, "DELETE", "/linked_accounts/123/", nil, http.StatusNotFound, api)
	})
	t.Run("InvalidAccountID", func(t *testing.T) {
		authToken := login("approved@resonant-kelpie-404a42.netlify.app", "")
		ServeRequest(t, authToken, "DELETE", "/linked_accounts/"+primitive.NewObjectID().Hex()+"/", nil, http.StatusNotFound, api)
	})
	t.Run("NotUnlinkableAccount", func(t *testing.T) {
		authToken := login("approved@resonant-kelpie-404a42.netlify.app", "")
		googleAccountID := getGoogleTokenFromAuthToken(t, api.DB, authToken).ID
		body := ServeRequest(t, authToken, "DELETE", "/linked_accounts/"+googleAccountID.Hex()+"/", nil, http.StatusBadRequest, api)
		assert.Equal(t, "{\"detail\":\"account is not unlinkable\"}", string(body))
	})
	t.Run("AccountDifferentUser", func(t *testing.T) {
		authToken := login("approved@resonant-kelpie-404a42.netlify.app", "")
		authTokenOther := login("other@resonant-kelpie-404a42.netlify.app", "")
		googleAccountID := getGoogleTokenFromAuthToken(t, api.DB, authTokenOther).ID
		ServeRequest(t, authToken, "DELETE", "/linked_accounts/"+googleAccountID.Hex()+"/", nil, http.StatusNotFound, api)
	})
	t.Run("Success", func(t *testing.T) {
		authToken := login("deletelinkedaccount@resonant-kelpie-404a42.netlify.app", "")
		linearTokenID := insertLinearToken(t, api.DB, authToken, false)
		ServeRequest(t, authToken, "DELETE", "/linked_accounts/"+linearTokenID.Hex()+"/", nil, http.StatusOK, api)
		var token database.ExternalAPIToken
		err := database.GetExternalTokenCollection(api.DB).FindOne(
			context.Background(),
			bson.M{"_id": linearTokenID},
		).Decode(&token)
		// assert token is not found in db anymore
		assert.Error(t, err)
	})
	t.Run("SuccessGithub", func(t *testing.T) {
		authToken := login("deletelinkedaccount_github@resonant-kelpie-404a42.netlify.app", "")
		userID := getUserIDFromAuthToken(t, api.DB, authToken)
		accountID := "correctAccountID"
		// should delete cached repos matching the account ID upon github unlink
		repositoryCollection := database.GetRepositoryCollection(api.DB)
		res, err := repositoryCollection.InsertOne(context.Background(), &database.Repository{UserID: userID, AccountID: accountID})
		assert.NoError(t, err)
		repoToDeleteID := res.InsertedID.(primitive.ObjectID)
		// should deleted cached repos with no account ID also (only affects accounts who have not recently refreshed PRs)
		res, err = repositoryCollection.InsertOne(context.Background(), &database.Repository{UserID: userID})
		assert.NoError(t, err)
		repoToDeleteID2 := res.InsertedID.(primitive.ObjectID)
		// wrong user id; shouldn't get deleted
		res, err = repositoryCollection.InsertOne(context.Background(), &database.Repository{UserID: primitive.NewObjectID()})
		assert.NoError(t, err)
		wrongUserRepoID := res.InsertedID.(primitive.ObjectID)
		// other account id; shouldn't get deleted
		res, err = repositoryCollection.InsertOne(context.Background(), &database.Repository{UserID: userID, AccountID: "notthisone"})
		assert.NoError(t, err)
		otherAccountIDRepoID := res.InsertedID.(primitive.ObjectID)
		res, err = database.GetExternalTokenCollection(api.DB).InsertOne(
			context.Background(),
			&database.ExternalAPIToken{
				AccountID:    accountID,
				ServiceID:    external.TASK_SERVICE_ID_GITHUB,
				UserID:       userID,
				DisplayID:    "Github",
				IsUnlinkable: true,
			},
		)
		assert.NoError(t, err)
		externalTokenID := res.InsertedID.(primitive.ObjectID)

		ServeRequest(t, authToken, "DELETE", "/linked_accounts/"+externalTokenID.Hex()+"/", nil, http.StatusOK, api)
		var token database.ExternalAPIToken
		err = database.GetExternalTokenCollection(api.DB).FindOne(
			context.Background(),
			bson.M{"_id": externalTokenID},
		).Decode(&token)
		// assert token is not found in db anymore
		assert.Error(t, err)

		var repository database.Repository
		err = repositoryCollection.FindOne(context.Background(), bson.M{"_id": repoToDeleteID}).Decode(&repository)
		// assert repo is not found in db anymore
		assert.Error(t, err)

		err = repositoryCollection.FindOne(context.Background(), bson.M{"_id": repoToDeleteID2}).Decode(&repository)
		// assert repo is not found in db anymore
		assert.Error(t, err)

		err = repositoryCollection.FindOne(context.Background(), bson.M{"_id": wrongUserRepoID}).Decode(&repository)
		// assert repo is found
		assert.NoError(t, err)

		err = repositoryCollection.FindOne(context.Background(), bson.M{"_id": otherAccountIDRepoID}).Decode(&repository)
		// assert repo is found
		assert.NoError(t, err)
	})
	t.Run("SuccessGoogle", func(t *testing.T) {
		authToken := login("deletelinkedaccount_github@resonant-kelpie-404a42.netlify.app", "")
		userID := getUserIDFromAuthToken(t, api.DB, authToken)
		notUserID := primitive.NewObjectID()
		accountID := "correctAccountID"

		calendarAccountToDelete, err := database.UpdateOrCreateCalendarAccount(api.DB, userID, "123abc", "foobar_source",
			&database.CalendarAccount{
				UserID:     userID,
				IDExternal: accountID,
				Calendars:  []database.Calendar{{CalendarID: "cal1", ColorID: "col1"}},
				Scopes:     []string{"https://www.googleapis.com/auth/calendar"},
			}, nil)
		assert.NoError(t, err)

		calendarAccountNotToDelete, err := database.UpdateOrCreateCalendarAccount(api.DB, notUserID, "123abc", "foobar_source",
			&database.CalendarAccount{
				UserID:     notUserID,
				IDExternal: "otherAccountID",
				Calendars:  []database.Calendar{{CalendarID: "cal2", ColorID: "col2"}},
				Scopes:     []string{"https://www.googleapis.com/auth/calendar"},
			}, nil)
		assert.NoError(t, err)

		res, err := database.GetExternalTokenCollection(api.DB).InsertOne(
			context.Background(),
			&database.ExternalAPIToken{
				AccountID:    accountID,
				ServiceID:    external.TASK_SERVICE_ID_GOOGLE,
				UserID:       userID,
				DisplayID:    "Google",
				IsUnlinkable: true,
			},
		)
		assert.NoError(t, err)
		externalTokenID := res.InsertedID.(primitive.ObjectID)

		ServeRequest(t, authToken, "DELETE", "/linked_accounts/"+externalTokenID.Hex()+"/", nil, http.StatusOK, api)
		var token database.ExternalAPIToken
		err = database.GetExternalTokenCollection(api.DB).FindOne(
			context.Background(),
			bson.M{"_id": externalTokenID},
		).Decode(&token)
		// assert token is not found in db anymore
		assert.Error(t, err)

		var account database.CalendarAccount
		// assert calendar account is not found in db anymore
		err = database.GetCalendarAccountCollection(api.DB).FindOne(context.Background(), bson.M{"_id": calendarAccountToDelete.ID}).Decode(&account)
		assert.Error(t, err)
		// assert other calendar account is still in the db
		err = database.GetCalendarAccountCollection(api.DB).FindOne(context.Background(), bson.M{"_id": calendarAccountNotToDelete.ID}).Decode(&account)
		assert.NoError(t, err)
		assert.Equal(t, "otherAccountID", account.IDExternal)
	})
	UnauthorizedTest(t, "DELETE", "/linked_accounts/123/", nil)
}

func insertLinearToken(t *testing.T, db *mongo.Database, authToken string, isBadToken bool) primitive.ObjectID {
	res, err := database.GetExternalTokenCollection(db).InsertOne(
		context.Background(),
		&database.ExternalAPIToken{
			ServiceID:    external.TASK_SERVICE_ID_LINEAR,
			UserID:       getUserIDFromAuthToken(t, db, authToken),
			DisplayID:    "Linear",
			IsUnlinkable: true,
			IsBadToken:   isBadToken,
		},
	)
	assert.NoError(t, err)
	return res.InsertedID.(primitive.ObjectID)
}
