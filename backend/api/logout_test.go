package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/franchizzle/task-manager/backend/database"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestLogout(t *testing.T) {
	UnauthorizedTest(t, "POST", "/logout/", nil)
	t.Run("Logout", func(t *testing.T) {
		authToken := login("approved@resonant-kelpie-404a42.netlify.app", "")

		db, dbCleanup, err := database.GetDBConnection()
		assert.NoError(t, err)
		defer dbCleanup()
		tokenCollection := database.GetInternalTokenCollection(db)

		count, _ := tokenCollection.CountDocuments(context.Background(), bson.M{"token": authToken})
		assert.Equal(t, int64(1), count)

		api, dbCleanup := GetAPIWithDBCleanup()
		defer dbCleanup()
		router := GetRouter(api)

		request, _ := http.NewRequest("POST", "/logout/", nil)
		request.Header.Add("Authorization", "Bearer "+authToken)

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusOK, recorder.Code)

		count, _ = tokenCollection.CountDocuments(context.Background(), bson.M{"token": authToken})
		assert.Equal(t, int64(0), count)
	})
}
