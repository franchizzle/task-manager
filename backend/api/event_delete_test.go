package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/franchizzle/task-manager/backend/testutils"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/franchizzle/task-manager/backend/database"
	"github.com/franchizzle/task-manager/backend/external"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestEventDelete(t *testing.T) {
	db, dbCleanup, err := database.GetDBConnection()
	assert.NoError(t, err)
	defer dbCleanup()

	authToken := login("approved@resonant-kelpie-404a42.netlify.app", "")
	userID := getUserIDFromAuthToken(t, db, authToken)

	eventCollection := database.GetCalendarEventCollection(db)

	insertResult, err := eventCollection.InsertOne(context.Background(), database.CalendarEvent{
		UserID:          userID,
		SourceAccountID: "account_id",
		CalendarID:      "cal_1",
		IDExternal:      "sample_calendar_id",
		SourceID:        external.TASK_SOURCE_ID_GCAL,
	})
	assert.NoError(t, err)
	insertResult2, err := eventCollection.InsertOne(context.Background(), database.CalendarEvent{
		UserID:          userID,
		SourceAccountID: "account_id",
		CalendarID:      "cal_2",
		IDExternal:      "sample_calendar_id",
		SourceID:        external.TASK_SOURCE_ID_GCAL,
	})
	assert.NoError(t, err)
	calendarTaskID := insertResult.InsertedID.(primitive.ObjectID)
	calendarTaskIDHex := calendarTaskID.Hex()
	calendarTaskID2 := insertResult2.InsertedID.(primitive.ObjectID)
	calendarTaskIDHex2 := calendarTaskID2.Hex()

	calendarDeleteServer := testutils.GetMockAPIServer(t, 200, "[]")
	api, dbCleanup := GetAPIWithDBCleanup()
	defer dbCleanup()
	api.ExternalConfig.GoogleOverrideURLs.CalendarDeleteURL = &calendarDeleteServer.URL
	router := GetRouter(api)

	UnauthorizedTest(t, "DELETE", "/events/delete/"+calendarTaskIDHex+"/", nil)
	t.Run("InvalidHex", func(t *testing.T) {
		ServeRequest(t, authToken, "DELETE", "/events/delete/"+calendarTaskIDHex+"1/", nil, http.StatusNotFound, nil)
	})

	t.Run("InvalidUser", func(t *testing.T) {
		secondAuthToken := login("tester@resonant-kelpie-404a42.netlify.app", "")
		ServeRequest(t, secondAuthToken, "DELETE", "/events/delete/"+calendarTaskIDHex+"1/", nil, http.StatusNotFound, nil)
	})

	t.Run("Success", func(t *testing.T) {
		var event database.CalendarEvent
		err = eventCollection.FindOne(context.Background(), bson.M{"_id": calendarTaskID}).Decode(&event)
		assert.Equal(t, "sample_calendar_id", event.IDExternal)

		request, _ := http.NewRequest(
			"DELETE",
			"/events/delete/"+calendarTaskIDHex+"/",
			bytes.NewBuffer([]byte(`{}`)))
		request.Header.Add("Authorization", "Bearer "+authToken)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusOK, recorder.Code)

		count, _ := eventCollection.CountDocuments(context.Background(), bson.M{"_id": calendarTaskID})
		assert.Equal(t, int64(0), count)
	})

	t.Run("SuccessDifferentEventWithSameAccountID", func(t *testing.T) {
		var event database.CalendarEvent
		err = eventCollection.FindOne(context.Background(), bson.M{"_id": calendarTaskID2}).Decode(&event)
		assert.Equal(t, "sample_calendar_id", event.IDExternal)

		request, _ := http.NewRequest(
			"DELETE",
			"/events/delete/"+calendarTaskIDHex2+"/",
			bytes.NewBuffer([]byte(`{}`)))
		request.Header.Add("Authorization", "Bearer "+authToken)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusOK, recorder.Code)

		count, _ := eventCollection.CountDocuments(context.Background(), bson.M{"_id": calendarTaskID2})
		assert.Equal(t, int64(0), count)
	})
}
