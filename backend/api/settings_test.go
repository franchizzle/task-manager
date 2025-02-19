package api

import (
	"bytes"
	"context"
	"github.com/franchizzle/task-manager/backend/constants"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/franchizzle/task-manager/backend/database"
	"github.com/franchizzle/task-manager/backend/settings"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestSettingsGet(t *testing.T) {
	api, dbCleanup := GetAPIWithDBCleanup()
	defer dbCleanup()
	settingCollection := database.GetUserSettingsCollection(api.DB)

	UnauthorizedTest(t, "GET", "/settings/", nil)
	t.Run("DefaultValue", func(t *testing.T) {
		// Random userID; should be ignored
		_, err := settingCollection.InsertOne(context.Background(), &database.UserSetting{
			UserID:     primitive.NewObjectID(),
			FieldKey:   constants.SettingFieldGithubFilteringPreference,
			FieldValue: constants.ChoiceKeyActionableOnly,
		})
		assert.NoError(t, err)

		authToken := login("approved@resonant-kelpie-404a42.netlify.app", "")
		api, dbCleanup := GetAPIWithDBCleanup()
		defer dbCleanup()
		router := GetRouter(api)
		request, _ := http.NewRequest("GET", "/settings/", nil)
		request.Header.Add("Authorization", "Bearer "+authToken)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusOK, recorder.Code)
		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Contains(t, string(body), "{\"field_key\":\"github_filtering_preference\",\"field_name\":\"\",\"choices\":[{\"choice_key\":\"actionable_only\",\"choice_name\":\"\"},{\"choice_key\":\"all_prs\",\"choice_name\":\"\"}],\"field_value\":\"actionable_only\"}")
	})
	t.Run("Success", func(t *testing.T) {
		authToken := login("approved@resonant-kelpie-404a42.netlify.app", "")
		userID := getUserIDFromAuthToken(t, api.DB, authToken)

		_, err := settingCollection.InsertOne(context.Background(), &database.UserSetting{
			UserID:     userID,
			FieldKey:   constants.SettingFieldGithubFilteringPreference,
			FieldValue: constants.ChoiceKeyActionableOnly,
		})
		assert.NoError(t, err)

		api, dbCleanup := GetAPIWithDBCleanup()
		defer dbCleanup()
		router := GetRouter(api)
		request, _ := http.NewRequest("GET", "/settings/", nil)
		request.Header.Add("Authorization", "Bearer "+authToken)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusOK, recorder.Code)
		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Contains(t, string(body), "{\"field_key\":\"github_filtering_preference\",\"field_name\":\"\",\"choices\":[{\"choice_key\":\"actionable_only\",\"choice_name\":\"\"},{\"choice_key\":\"all_prs\",\"choice_name\":\"\"}],\"field_value\":\"actionable_only\"}")
	})
	UnauthorizedTest(t, "GET", "/settings/", nil)
	t.Run("Unauthorized", func(t *testing.T) {
		router := GetRouter(api)
		request, _ := http.NewRequest("GET", "/settings/", nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	})
}

func TestSettingsModify(t *testing.T) {
	db, dbCleanup, err := database.GetDBConnection()
	assert.NoError(t, err)
	defer dbCleanup()

	t.Run("EmptyPayload", func(t *testing.T) {
		authToken := login("approved@resonant-kelpie-404a42.netlify.app", "")
		api, dbCleanup := GetAPIWithDBCleanup()
		defer dbCleanup()
		router := GetRouter(api)
		request, _ := http.NewRequest("PATCH", "/settings/", nil)
		request.Header.Add("Authorization", "Bearer "+authToken)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Equal(t, "{\"detail\":\"parameters missing or malformatted.\"}", string(body))
	})
	t.Run("InvalidPayload", func(t *testing.T) {
		authToken := login("approved@resonant-kelpie-404a42.netlify.app", "")
		api, dbCleanup := GetAPIWithDBCleanup()
		defer dbCleanup()
		router := GetRouter(api)
		request, _ := http.NewRequest(
			"PATCH",
			"/settings/",
			bytes.NewBuffer([]byte(`["not", "a", "map"]`)),
		)
		request.Header.Add("Authorization", "Bearer "+authToken)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Equal(t, "{\"detail\":\"parameters missing or malformatted.\"}", string(body))
	})
	t.Run("BadKey", func(t *testing.T) {
		authToken := login("approved@resonant-kelpie-404a42.netlify.app", "")
		api, dbCleanup := GetAPIWithDBCleanup()
		defer dbCleanup()
		router := GetRouter(api)
		request, _ := http.NewRequest(
			"PATCH",
			"/settings/",
			bytes.NewBuffer([]byte(`{"dogecoin": "tothemoon"}`)),
		)
		request.Header.Add("Authorization", "Bearer "+authToken)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Equal(t, "{\"detail\":\"failed to update settings: invalid setting: dogecoin\"}", string(body))
	})
	t.Run("BadValue", func(t *testing.T) {
		authToken := login("approved@resonant-kelpie-404a42.netlify.app", "")
		api, dbCleanup := GetAPIWithDBCleanup()
		defer dbCleanup()
		router := GetRouter(api)
		request, _ := http.NewRequest(
			"PATCH",
			"/settings/",
			bytes.NewBuffer([]byte(`{"github_filtering_preference": "tothemoon"}`)),
		)
		request.Header.Add("Authorization", "Bearer "+authToken)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Equal(t, "{\"detail\":\"failed to update settings: invalid value: tothemoon\"}", string(body))
	})
	t.Run("Success", func(t *testing.T) {
		authToken := login("approved@resonant-kelpie-404a42.netlify.app", "")
		api, dbCleanup := GetAPIWithDBCleanup()
		defer dbCleanup()
		router := GetRouter(api)
		request, _ := http.NewRequest(
			"PATCH",
			"/settings/",
			bytes.NewBuffer([]byte(`{"github_filtering_preference": "all_prs"}`)),
		)
		request.Header.Add("Authorization", "Bearer "+authToken)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusOK, recorder.Code)
		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Equal(t, "{}", string(body))

		request, _ = http.NewRequest("GET", "/settings/", nil)
		request.Header.Add("Authorization", "Bearer "+authToken)
		recorder = httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusOK, recorder.Code)
		body, err = io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Contains(t, string(body), "{\"field_key\":\"github_filtering_preference\",\"field_name\":\"\",\"choices\":[{\"choice_key\":\"actionable_only\",\"choice_name\":\"\"},{\"choice_key\":\"all_prs\",\"choice_name\":\"\"}],\"field_value\":\"all_prs\"}")
	})
	t.Run("SuccessAlreadyExists", func(t *testing.T) {
		authToken := login("approved2@resonant-kelpie-404a42.netlify.app", "")
		userID := getUserIDFromAuthToken(t, db, authToken)
		settings.UpdateUserSetting(db, userID, constants.SettingFieldGithubFilteringPreference, constants.ChoiceKeyActionableOnly)

		api, dbCleanup := GetAPIWithDBCleanup()
		defer dbCleanup()
		router := GetRouter(api)
		request, _ := http.NewRequest(
			"PATCH",
			"/settings/",
			bytes.NewBuffer([]byte(`{"github_filtering_preference": "all_prs"}`)),
		)
		request.Header.Add("Authorization", "Bearer "+authToken)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusOK, recorder.Code)
		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Equal(t, "{}", string(body))

		request, _ = http.NewRequest("GET", "/settings/", nil)
		request.Header.Add("Authorization", "Bearer "+authToken)
		recorder = httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusOK, recorder.Code)
		body, err = io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Contains(t, string(body), "{\"field_key\":\"github_filtering_preference\",\"field_name\":\"\",\"choices\":[{\"choice_key\":\"actionable_only\",\"choice_name\":\"\"},{\"choice_key\":\"all_prs\",\"choice_name\":\"\"}],\"field_value\":\"all_prs\"}")
	})
}
