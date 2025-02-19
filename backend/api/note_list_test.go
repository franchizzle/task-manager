package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/franchizzle/task-manager/backend/database"
	"github.com/franchizzle/task-manager/backend/testutils"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestNotesList(t *testing.T) {
	authToken := login("test_notes_list@resonant-kelpie-404a42.netlify.app", "")
	title1 := "title1"
	title2 := "title2"
	title3 := "deleted note"
	db, dbCleanup, err := database.GetDBConnection()
	assert.NoError(t, err)
	defer dbCleanup()
	userID := getUserIDFromAuthToken(t, db, authToken)
	notUserID := primitive.NewObjectID()
	task1, err := database.GetOrCreateNote(
		db,
		userID,
		"123abc",
		"foobar_source",
		&database.Note{
			UserID:      userID,
			Title:       &title1,
			SharedUntil: *testutils.CreateDateTime("9999-01-01"),
		},
	)
	assert.NoError(t, err)
	task2, err := database.GetOrCreateNote(
		db,
		userID,
		"123abcdef",
		"foobar_source",
		&database.Note{
			UserID:      userID,
			Title:       &title2,
			SharedUntil: *testutils.CreateDateTime("1999-01-01"),
		},
	)
	assert.NoError(t, err)
	_, err = database.GetOrCreateNote(
		db,
		userID,
		"123abc",
		"foobar_source",
		&database.Note{
			UserID:      notUserID,
			SharedUntil: *testutils.CreateDateTime("9999-01-01"),
		},
	)
	assert.NoError(t, err)
	isDeleted := true
	domain := database.SharedAccessDomain
	task3, err := database.GetOrCreateNote(
		db,
		userID,
		"123abcdogecoin",
		"foobar_source",
		&database.Note{
			UserID:       userID,
			Title:        &title3,
			SharedUntil:  *testutils.CreateDateTime("9999-01-01"),
			IsDeleted:    &isDeleted,
			SharedAccess: &domain,
		},
	)
	assert.NoError(t, err)

	UnauthorizedTest(t, "GET", "/notes/", nil)
	t.Run("Success", func(t *testing.T) {
		api, dbCleanup := GetAPIWithDBCleanup()
		defer dbCleanup()

		response := ServeRequest(t, authToken, "GET", "/notes/?", nil, http.StatusOK, api)
		var result []NoteResult
		err = json.Unmarshal(response, &result)

		assert.NoError(t, err)
		assert.Equal(t, 3, len(result))
		assert.Equal(t, []NoteResult{
			{
				ID:          task1.ID,
				Title:       "title1",
				SharedUntil: "9999-01-01T00:00:00Z",
				CreatedAt:   "1970-01-01T00:00:00Z",
				UpdatedAt:   "1970-01-01T00:00:00Z",
			},
			{
				ID:          task2.ID,
				Title:       "title2",
				SharedUntil: "1999-01-01T00:00:00Z",
				CreatedAt:   "1970-01-01T00:00:00Z",
				UpdatedAt:   "1970-01-01T00:00:00Z",
			},
			{
				ID:           task3.ID,
				Title:        "deleted note",
				SharedUntil:  "9999-01-01T00:00:00Z",
				CreatedAt:    "1970-01-01T00:00:00Z",
				UpdatedAt:    "1970-01-01T00:00:00Z",
				IsDeleted:    true,
				SharedAccess: "domain",
			},
		}, result)
	})
}
