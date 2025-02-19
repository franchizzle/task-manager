package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/franchizzle/task-manager/backend/testutils"

	"github.com/franchizzle/task-manager/backend/external"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/googleapi"

	"github.com/franchizzle/task-manager/backend/database"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestEventList(t *testing.T) {
	api, dbCleanup := GetAPIWithDBCleanup()
	defer dbCleanup()

	// set up tokens: one bad, one good, one wrong user id

	// set up a bunch of events in the DB
	// wrong user id, wrong account id, wrong start time, wrong end time, one with a linked task, one with a linekd view
	// set up mock server to return two of those events (to verify sorting)
	// validate correct events are deleted

	// additional cases: 1) unauthorized, 2) bad params, 3) external failure

	beforeStartTime, _ := time.Parse(time.RFC3339, "2021-03-06T14:59:00-05:00")
	startTime, _ := time.Parse(time.RFC3339, "2021-03-06T15:00:00-05:00")
	endTime, _ := time.Parse(time.RFC3339, "2021-03-06T15:30:00-05:00")
	afterEndTime, _ := time.Parse(time.RFC3339, "2021-03-06T15:30:01-05:00")
	sourceAccountID := "TestEventList@resonant-kelpie-404a42.netlify.app"
	authToken := login("TestEventList@resonant-kelpie-404a42.netlify.app", "")
	userID := getUserIDFromAuthToken(t, api.DB, authToken)

	externalTokenCollection := database.GetExternalTokenCollection(api.DB)
	// wrong service
	_, err := externalTokenCollection.InsertOne(
		context.Background(),
		&database.ExternalAPIToken{
			AccountID: sourceAccountID,
			ServiceID: external.TASK_SERVICE_ID_LINEAR,
			UserID:    userID,
		},
	)
	assert.NoError(t, err)
	// wrong user id
	_, err = externalTokenCollection.InsertOne(
		context.Background(),
		&database.ExternalAPIToken{
			AccountID: sourceAccountID,
			ServiceID: external.TASK_SERVICE_ID_GOOGLE,
			UserID:    primitive.NewObjectID(),
		},
	)
	assert.NoError(t, err)
	// bad token
	_, err = externalTokenCollection.InsertOne(
		context.Background(),
		&database.ExternalAPIToken{
			AccountID:  sourceAccountID,
			ServiceID:  external.TASK_SERVICE_ID_GOOGLE,
			UserID:     userID,
			IsBadToken: true,
		},
	)
	assert.NoError(t, err)

	eventCollection := database.GetCalendarEventCollection(api.DB)
	_, err = eventCollection.InsertOne(context.Background(), database.CalendarEvent{
		Title:           "Normal Event",
		IDExternal:      "normal_event",
		SourceAccountID: sourceAccountID,
		SourceID:        external.TASK_SOURCE_ID_GCAL,
		UserID:          userID,
		DatetimeStart:   primitive.NewDateTimeFromTime(startTime),
		DatetimeEnd:     primitive.NewDateTimeFromTime(endTime),
	})
	assert.NoError(t, err)
	_, err = eventCollection.InsertOne(context.Background(), database.CalendarEvent{
		Title:           "Normal Event 2",
		IDExternal:      "normal_event2",
		SourceAccountID: sourceAccountID,
		SourceID:        external.TASK_SOURCE_ID_GCAL,
		UserID:          userID,
		DatetimeStart:   primitive.NewDateTimeFromTime(startTime),
		DatetimeEnd:     primitive.NewDateTimeFromTime(endTime),
	})
	assert.NoError(t, err)
	// wrong user id
	_, err = eventCollection.InsertOne(context.Background(), database.CalendarEvent{
		Title:           "Wrong User ID",
		IDExternal:      "wrong_user_id",
		SourceAccountID: sourceAccountID,
		SourceID:        external.TASK_SOURCE_ID_GCAL,
		UserID:          primitive.NewObjectID(),
		DatetimeStart:   primitive.NewDateTimeFromTime(startTime),
		DatetimeEnd:     primitive.NewDateTimeFromTime(endTime),
	})
	assert.NoError(t, err)
	// wrong account id
	_, err = eventCollection.InsertOne(context.Background(), database.CalendarEvent{
		Title:           "Wrong Account ID",
		IDExternal:      "wrong_account_id",
		SourceAccountID: "oopsie whoopsie",
		SourceID:        external.TASK_SOURCE_ID_GCAL,
		UserID:          userID,
		DatetimeStart:   primitive.NewDateTimeFromTime(startTime),
		DatetimeEnd:     primitive.NewDateTimeFromTime(endTime),
	})
	assert.NoError(t, err)
	// wrong start time
	_, err = eventCollection.InsertOne(context.Background(), database.CalendarEvent{
		Title:           "Wrong Start Time",
		IDExternal:      "wrong_start_time",
		SourceAccountID: sourceAccountID,
		SourceID:        external.TASK_SOURCE_ID_GCAL,
		UserID:          userID,
		DatetimeStart:   primitive.NewDateTimeFromTime(afterEndTime),
		DatetimeEnd:     primitive.NewDateTimeFromTime(endTime),
	})
	assert.NoError(t, err)
	// wrong end time
	_, err = eventCollection.InsertOne(context.Background(), database.CalendarEvent{
		Title:           "Wrong End Time",
		IDExternal:      "wrong_end_time",
		SourceAccountID: sourceAccountID,
		SourceID:        external.TASK_SOURCE_ID_GCAL,
		UserID:          userID,
		DatetimeStart:   primitive.NewDateTimeFromTime(startTime),
		DatetimeEnd:     primitive.NewDateTimeFromTime(beforeStartTime),
	})
	assert.NoError(t, err)

	UnauthorizedTest(t, "GET", "/events/", nil)
	t.Run("MissingParameter", func(t *testing.T) {
		response := ServeRequest(t, authToken, "GET", "/events/", nil, http.StatusBadRequest, api)
		assert.Equal(t, "{\"detail\":\"invalid or missing parameter.\"}", string(response))
	})
	t.Run("BadParameter", func(t *testing.T) {
		params := url.Values{}
		params.Add("datetime_start", "oof")
		params.Add("datetime_end", "ooooof")
		response := ServeRequest(t, authToken, "GET", "/events/?"+params.Encode(), nil, http.StatusBadRequest, api)
		assert.Equal(t, "{\"detail\":\"invalid or missing parameter.\"}", string(response))
	})
	t.Run("FailedToLoadToken", func(t *testing.T) {
		params := url.Values{}
		params.Add("datetime_start", "2021-03-06T15:00:00-05:00")
		params.Add("datetime_end", "2021-03-06T15:30:00-05:00")
		response := ServeRequest(t, authToken, "GET", "/events/?"+params.Encode(), nil, http.StatusOK, api)
		assert.Equal(t, "[]", string(response))

		// no events should be deleted because events call failed
		count, err := eventCollection.CountDocuments(context.Background(), bson.M{"user_id": userID})
		assert.NoError(t, err)
		assert.Equal(t, int64(5), count)
	})
	t.Run("Success", func(t *testing.T) {
		oooEvent := calendar.Event{
			Created:         "2023-02-25T17:53:01.000Z",
			Summary:         "ooo Event",
			Description:     "ooo event <strong>description</strong>",
			Location:        "ooo event location",
			EventType:       "outOfOffice",
			Start:           &calendar.EventDateTime{DateTime: "2023-03-06T15:01:00-05:00"},
			End:             &calendar.EventDateTime{DateTime: "2023-03-06T15:30:00-05:00"},
			HtmlLink:        "resonant-kelpie-404a42.netlify.app",
			Id:              "ooo_event",
			GuestsCanModify: false,
			Organizer:       &calendar.EventOrganizer{Self: true},
			ServerResponse:  googleapi.ServerResponse{HTTPStatusCode: 0},
		}
		standardEvent := calendar.Event{
			Created:         "2021-02-25T17:53:01.000Z",
			Summary:         "Normal Event",
			Description:     "event <strong>description</strong>",
			Location:        "normal event location",
			Start:           &calendar.EventDateTime{DateTime: "2021-03-06T15:01:00-05:00"},
			End:             &calendar.EventDateTime{DateTime: "2021-03-06T15:30:00-05:00"},
			HtmlLink:        "resonant-kelpie-404a42.netlify.app",
			Id:              "normal_event",
			GuestsCanModify: false,
			Organizer:       &calendar.EventOrganizer{Self: true},
			ServerResponse:  googleapi.ServerResponse{HTTPStatusCode: 0},
		}
		// new event should come first in the results because it has an earlier start time
		newEvent := calendar.Event{
			Created:         "2021-02-25T17:53:01.000Z",
			Summary:         "New Event",
			Description:     "event <strong>description</strong>",
			Location:        "new event location",
			Start:           &calendar.EventDateTime{DateTime: "2021-03-06T15:00:00-05:00"},
			End:             &calendar.EventDateTime{DateTime: "2021-03-06T15:30:00-05:00"},
			HtmlLink:        "resonant-kelpie-404a42.netlify.app",
			Id:              "new_event",
			GuestsCanModify: false,
			Organizer:       &calendar.EventOrganizer{Self: true},
			ServerResponse:  googleapi.ServerResponse{HTTPStatusCode: 0},
		}
		server := testutils.GetGcalFetchServer([]*calendar.Event{&standardEvent, &newEvent, &oooEvent})
		defer server.Close()
		api.ExternalConfig.GoogleOverrideURLs.CalendarFetchURL = &server.URL

		params := url.Values{}
		params.Add("datetime_start", "2021-03-06T15:00:00-05:00")
		params.Add("datetime_end", "2021-03-06T15:30:00-05:00")
		response := ServeRequest(t, authToken, "GET", "/events/?"+params.Encode(), nil, http.StatusOK, api)
		var eventResult []EventResult
		err = json.Unmarshal(response, &eventResult)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(eventResult))
		assert.Equal(t, "New Event", eventResult[0].Title)
		assert.Equal(t, "Normal Event", eventResult[1].Title)
		// ooo event should not be in result

		// normal_event2 should be deleted and replaced by new_event
		count, err := eventCollection.CountDocuments(context.Background(), bson.M{"user_id": userID})
		assert.NoError(t, err)
		assert.Equal(t, int64(6), count)
		count, err = eventCollection.CountDocuments(context.Background(), bson.M{"$and": []bson.M{
			{"user_id": userID},
			{"id_external": "normal_event2"},
		}})
		assert.NoError(t, err)
		assert.Equal(t, int64(0), count)
		count, err = eventCollection.CountDocuments(context.Background(), bson.M{"$and": []bson.M{
			{"user_id": userID},
			{"id_external": "new_event"},
		}})
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})
}

func TestEventDetail(t *testing.T) {
	api, dbCleanup := GetAPIWithDBCleanup()
	defer dbCleanup()

	startTime, _ := time.Parse(time.RFC3339, "2021-03-06T15:00:00-05:00")
	endTime, _ := time.Parse(time.RFC3339, "2021-03-06T15:30:00-05:00")
	sourceAccountID := "TestEventDetail@resonant-kelpie-404a42.netlify.app"
	authToken := login("TestEventDetail@resonant-kelpie-404a42.netlify.app", "")
	userID := getUserIDFromAuthToken(t, api.DB, authToken)

	eventCollection := database.GetCalendarEventCollection(api.DB)
	calendarEvent := database.CalendarEvent{
		Title:           "Normal Event",
		IDExternal:      "normal_event",
		SourceAccountID: sourceAccountID,
		SourceID:        external.TASK_SOURCE_ID_GCAL,
		UserID:          userID,
		DatetimeStart:   primitive.NewDateTimeFromTime(startTime),
		DatetimeEnd:     primitive.NewDateTimeFromTime(endTime),
	}
	insertResult, err := eventCollection.InsertOne(context.Background(), calendarEvent)
	assert.NoError(t, err)
	eventID := insertResult.InsertedID.(primitive.ObjectID)

	UnauthorizedTest(t, "GET", "/events/", nil)
	t.Run("WrongID", func(t *testing.T) {
		eventID2 := primitive.NewObjectID()
		_ = ServeRequest(t, authToken, "GET", "/events/"+eventID2.Hex()+"/", nil, http.StatusNotFound, api)
	})
	t.Run("WrongUser", func(t *testing.T) {
		authToken2 := login("wronguserforeventdetails@resonant-kelpie-404a42.netlify.app", "")
		_ = ServeRequest(t, authToken2, "GET", "/events/"+eventID.Hex()+"/", nil, http.StatusNotFound, api)
	})
	t.Run("Success", func(t *testing.T) {
		expectedResult, err := api.calendarEventToResult(&calendarEvent, userID)
		assert.NoError(t, err)

		output := ServeRequest(t, authToken, "GET", "/events/"+eventID.Hex()+"/", nil, http.StatusOK, api)
		var eventResult EventResult
		err = json.Unmarshal(output, &eventResult)

		assert.Equal(t, expectedResult.DatetimeStart, eventResult.DatetimeStart)
		assert.Equal(t, expectedResult.DatetimeEnd, eventResult.DatetimeEnd)
		assert.Equal(t, expectedResult.Deeplink, eventResult.Deeplink)
		assert.Equal(t, expectedResult.CalendarID, eventResult.CalendarID)
		assert.Equal(t, expectedResult.Title, eventResult.Title)
		assert.Equal(t, expectedResult.Body, eventResult.Body)
		assert.Equal(t, expectedResult.Location, eventResult.Location)
	})
}

func TestGetLinkedNoteID(t *testing.T) {
	api, dbCleanup := GetAPIWithDBCleanup()
	defer dbCleanup()

	userID := primitive.NewObjectID()
	eventID := primitive.NewObjectID()

	insertResult, err := database.GetOrCreateNote(
		api.DB,
		userID,
		"external_id",
		"source_id",
		&database.Note{
			UserID:        userID,
			Author:        "author1",
			CreatedAt:     *testutils.CreateDateTime("2020-04-20"),
			UpdatedAt:     *testutils.CreateDateTime("2020-04-20"),
			SharedUntil:   *testutils.CreateDateTime("9999-01-01"),
			LinkedEventID: eventID,
		},
	)
	assert.NoError(t, err)
	noteID := insertResult.ID

	t.Run("WrongID", func(t *testing.T) {
		testID := primitive.NewObjectID()
		noteIDHex := api.getLinkedNoteID(testID, userID)
		assert.Equal(t, "", noteIDHex)
	})
	t.Run("Success", func(t *testing.T) {
		noteIDHex := api.getLinkedNoteID(eventID, userID)
		assert.Equal(t, noteID.Hex(), noteIDHex)
	})
}
