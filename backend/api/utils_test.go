package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/franchizzle/task-manager/backend/database"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestCORSHeaders(t *testing.T) {
	api, dbCleanup := GetAPIWithDBCleanup()
	defer dbCleanup()
	router := GetRouter(api)
	t.Run("OPTIONS preflight request default", func(t *testing.T) {
		request, _ := http.NewRequest("OPTIONS", "/tasks/", nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusNoContent, recorder.Code)
		headers := recorder.Result().Header
		assert.Equal(t, "Authorization,Access-Control-Allow-Origin,Access-Control-Allow-Headers,Access-Control-Allow-Methods,Content-Type,Timezone-Offset,sentry-trace,baggage",
			headers.Get("Access-Control-Allow-Headers"))
		assert.Equal(t, "http://localhost:3000", headers.Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "POST, OPTIONS, GET, PUT, PATCH, DELETE", headers.Get("Access-Control-Allow-Methods"))
	})
	t.Run("OPTIONS preflight request mobile", func(t *testing.T) {
		request, _ := http.NewRequest("OPTIONS", "/tasks/", nil)
		request.Header.Add("Origin", "http://mobile.localhost.com:3000")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusNoContent, recorder.Code)
		headers := recorder.Result().Header
		assert.Equal(t, "Authorization,Access-Control-Allow-Origin,Access-Control-Allow-Headers,Access-Control-Allow-Methods,Content-Type,Timezone-Offset,sentry-trace,baggage",
			headers.Get("Access-Control-Allow-Headers"))
		assert.Equal(t, "http://mobile.localhost.com:3000", headers.Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "POST, OPTIONS, GET, PUT, PATCH, DELETE", headers.Get("Access-Control-Allow-Methods"))
	})
	t.Run("GET request default", func(t *testing.T) {
		request, _ := http.NewRequest("GET", "/ping_authed/", nil)
		authToken := login("approved@resonant-kelpie-404a42.netlify.app", "")
		request.Header.Add("Authorization", "Bearer "+authToken)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusOK, recorder.Code)
		headers := recorder.Result().Header
		assert.Equal(t, "Authorization,Access-Control-Allow-Origin,Access-Control-Allow-Headers,Access-Control-Allow-Methods,Content-Type,Timezone-Offset,sentry-trace,baggage",
			headers.Get("Access-Control-Allow-Headers"))
		assert.Equal(t, "http://localhost:3000", headers.Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "POST, OPTIONS, GET, PUT, PATCH, DELETE", headers.Get("Access-Control-Allow-Methods"))
	})
	t.Run("GET request default mobile", func(t *testing.T) {
		request, _ := http.NewRequest("GET", "/ping_authed/", nil)
		authToken := login("approved@resonant-kelpie-404a42.netlify.app", "")
		request.Header.Add("Authorization", "Bearer "+authToken)
		request.Header.Add("Origin", "http://mobile.localhost.com:3000")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusOK, recorder.Code)
		headers := recorder.Result().Header
		assert.Equal(t, "Authorization,Access-Control-Allow-Origin,Access-Control-Allow-Headers,Access-Control-Allow-Methods,Content-Type,Timezone-Offset,sentry-trace,baggage",
			headers.Get("Access-Control-Allow-Headers"))
		assert.Equal(t, "http://mobile.localhost.com:3000", headers.Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "POST, OPTIONS, GET, PUT, PATCH, DELETE", headers.Get("Access-Control-Allow-Methods"))
	})
}

func TestAuthenticationMiddleware(t *testing.T) {
	authToken := login("approved@resonant-kelpie-404a42.netlify.app", "")

	t.Run("InvalidLength", func(t *testing.T) {
		recorder := runAuthenticatedEndpoint("Bearer hello")
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
		recorder = runAuthenticatedEndpoint("hello")
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
		recorder = runAuthenticatedEndpoint(authToken)
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Equal(t, "{\"detail\":\"incorrect auth token format\"}", string(body))
	})

	t.Run("InvalidToken", func(t *testing.T) {
		recorder := runAuthenticatedEndpoint("Bearer c5b034f4-a645-4352-91d6-0c271afc4076")
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Equal(t, "{\"detail\":\"unauthorized\"}", string(body))
	})

	t.Run("Valid", func(t *testing.T) {
		recorder := runAuthenticatedEndpoint("Bearer " + authToken)
		assert.Equal(t, http.StatusOK, recorder.Code)
		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Equal(t, "\"success\"", string(body))
	})
}

func TestBusinessMiddleware(t *testing.T) {
	authToken := login("test_business_middleware@resonant-kelpie-404a42.netlify.app", "")

	t.Run("Forbidden", func(t *testing.T) {
		recorder := runBusinessEndpoint("Bearer " + authToken)
		assert.Equal(t, http.StatusForbidden, recorder.Code)
		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Equal(t, "{\"detail\":\"business access is required to use this endpoint\"}", string(body))
	})

	t.Run("Success", func(t *testing.T) {
		// update to enable business mode flag
		db, dbCleanup, err := database.GetDBConnection()
		assert.NoError(t, err)
		defer dbCleanup()
		userID := getUserIDFromAuthToken(t, db, authToken)
		_, err = database.GetUserCollection(db).UpdateOne(context.Background(), bson.M{"_id": userID}, bson.M{"$set": bson.M{"business_mode_enabled": true}})
		assert.NoError(t, err)

		recorder := runBusinessEndpoint("Bearer " + authToken)
		assert.Equal(t, http.StatusOK, recorder.Code)
		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Equal(t, "\"success\"", string(body))
	})
}

func TestLoggingMiddleware(t *testing.T) {
	authToken := login("approved@resonant-kelpie-404a42.netlify.app", "")
	t.Run("Success", func(t *testing.T) {
		api, dbCleanup := GetAPIWithDBCleanup()
		defer dbCleanup()
		router := GetRouter(api)

		request, _ := http.NewRequest("GET", "/bing_bong/", nil)
		request.Header.Add("Authorization", authToken)

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)

		db, dbCleanup, err := database.GetDBConnection()
		assert.NoError(t, err)
		defer dbCleanup()
		count, err := database.GetLogEventsCollection(db).CountDocuments(
			context.Background(),
			bson.M{"event_type": "api_hit_/bing_bong/"},
		)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})
	t.Run("DoesntRecordForLogEndpoint", func(t *testing.T) {
		api, dbCleanup := GetAPIWithDBCleanup()
		defer dbCleanup()
		router := GetRouter(api)

		request, _ := http.NewRequest("POST", "/log_events/", nil)
		request.Header.Add("Authorization", authToken)

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)

		db, dbCleanup, err := database.GetDBConnection()
		assert.NoError(t, err)
		defer dbCleanup()
		count, err := database.GetLogEventsCollection(db).CountDocuments(
			context.Background(),
			bson.M{"event_type": "api_hit_/log_events/"},
		)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})
	t.Run("Unauthorized", func(t *testing.T) {
		api, dbCleanup := GetAPIWithDBCleanup()
		defer dbCleanup()
		router := GetRouter(api)

		request, _ := http.NewRequest("GET", "/bing_bong_two/", nil)

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)

		db, dbCleanup, err := database.GetDBConnection()
		assert.NoError(t, err)
		defer dbCleanup()
		count, err := database.GetLogEventsCollection(db).CountDocuments(
			context.Background(),
			bson.M{"event_type": "api_hit_/bing_bong_two/"},
		)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})
}

func Test404(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		api, dbCleanup := GetAPIWithDBCleanup()
		defer dbCleanup()
		router := GetRouter(api)
		request, _ := http.NewRequest("GET", "/not/a-route/", nil)

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusNotFound, recorder.Code)
		body, err := io.ReadAll(recorder.Body)
		assert.NoError(t, err)
		assert.Equal(t, "{\"detail\":\"not found\"}", string(body))
	})
}

func TestIsLocalServer(t *testing.T) {
	assert.False(t, isLocalServer())
}

func TestLogRequestMiddleware(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		api, dbCleanup := GetAPIWithDBCleanup()
		defer dbCleanup()
		router := GetRouter(api)
		request, _ := http.NewRequest("GET", "/not/a-route-2/", nil)

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusNotFound, recorder.Code)

		serverRequestsCollection := database.GetServerRequestCollection(api.DB)
		count, err := serverRequestsCollection.CountDocuments(
			context.Background(),
			bson.M{"method": "/not/a-route-2/"},
		)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})
}
