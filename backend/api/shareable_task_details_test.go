package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"testing"

	"github.com/franchizzle/task-manager/backend/database"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestShareableTaskDetails(t *testing.T) {
	authToken := login("test_shareable_task_details@resonant-kelpie-404a42.netlify.app", "")
	db, dbCleanup, err := database.GetDBConnection()
	assert.NoError(t, err)
	defer dbCleanup()
	api, dbCleanup := GetAPIWithDBCleanup()
	defer dbCleanup()

	taskCollection := database.GetTaskCollection(db)
	userID := getUserIDFromAuthToken(t, db, authToken)

	publicSharedAccess := database.SharedAccessPublic
	domainSharedAccess := database.SharedAccessDomain

	futureTime := primitive.NewDateTimeFromTime(time.Now().Add(1 * time.Hour))
	expiredTime := primitive.NewDateTimeFromTime(time.Now().Add(-1 * time.Hour))

	// Create a task that is not shared
	mongoResult, err := taskCollection.InsertOne(context.Background(), &database.Task{
		UserID: userID,
	})
	assert.NoError(t, err)
	notSharedTaskID := mongoResult.InsertedID.(primitive.ObjectID).Hex()

	// Create a task that is shared with everyone
	mongoResult, err = taskCollection.InsertOne(context.Background(), &database.Task{
		UserID:       userID,
		SharedUntil:  futureTime,
		SharedAccess: &publicSharedAccess,
	})
	assert.NoError(t, err)
	publicSharedTaskID := mongoResult.InsertedID.(primitive.ObjectID).Hex()

	// Create a task that is shared with everyone with same domain
	mongoResult, err = taskCollection.InsertOne(context.Background(), &database.Task{
		UserID:       userID,
		SharedUntil:  futureTime,
		SharedAccess: &domainSharedAccess,
	})
	assert.NoError(t, err)
	domainSharedTaskID := mongoResult.InsertedID.(primitive.ObjectID).Hex()

	// Create task with expired sharedUntil
	mongoResult, err = taskCollection.InsertOne(context.Background(), &database.Task{
		UserID:       userID,
		SharedUntil:  expiredTime,
		SharedAccess: &publicSharedAccess,
	})
	assert.NoError(t, err)
	expiredTaskID := mongoResult.InsertedID.(primitive.ObjectID).Hex()

	t.Run("InvalidTaskID", func(t *testing.T) {
		ServeRequest(t, authToken, "GET", fmt.Sprintf("/shareable_tasks/detail/%s/", primitive.NewObjectID().Hex()), nil, 404, api)
	})
	t.Run("TaskNotShared", func(t *testing.T) {
		ServeRequest(t, authToken, "GET", fmt.Sprintf("/shareable_tasks/detail/%s/", notSharedTaskID), nil, 404, api)
	})
	t.Run("SuccessPublic", func(t *testing.T) {
		differentDomainUserToken := login("differentDomain@applesauce.com", "")
		ServeRequest(t, differentDomainUserToken, "GET", fmt.Sprintf("/shareable_tasks/detail/%s/", publicSharedTaskID), nil, 200, api)
	})
	t.Run("SuccessDomain", func(t *testing.T) {
		sameDomain := login("sameDomainDifferentUser@resonant-kelpie-404a42.netlify.app", "")
		ServeRequest(t, sameDomain, "GET", fmt.Sprintf("/shareable_tasks/detail/%s/", domainSharedTaskID), nil, 200, api)
	})
	t.Run("DifferentDomain", func(t *testing.T) {
		differentDomainUserToken := login("wrongDomain@applesauce.com", "")
		ServeRequest(t, differentDomainUserToken, "GET", fmt.Sprintf("/shareable_tasks/detail/%s/", domainSharedTaskID), nil, 404, api)
	})
	t.Run("TaskSharedTimeExpired", func(t *testing.T) {
		ServeRequest(t, authToken, "GET", fmt.Sprintf("/shareable_tasks/detail/%s/", expiredTaskID), nil, 404, api)
	})
	t.Run("SuccessPublicUnauthorizedUser", func(t *testing.T) {
		ServeRequest(t, "", "GET", fmt.Sprintf("/shareable_tasks/detail/%s/", publicSharedTaskID), nil, 200, api)
	})
	t.Run("DomainUnauthorizedUser", func(t *testing.T) {
		ServeRequest(t, "", "GET", fmt.Sprintf("/shareable_tasks/detail/%s/", domainSharedTaskID), nil, 404, api)
	})
	t.Run("NoSubtasks", func(t *testing.T) {
		response := ServeRequest(t, authToken, "GET", fmt.Sprintf("/shareable_tasks/detail/%s/", publicSharedTaskID), nil, 200, api)
		var result ShareableTaskDetailsResponse
		err := json.Unmarshal(response, &result)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(result.Subtasks))
	})
	t.Run("WithSubtasks", func(t *testing.T) {
		mongoResult, err = taskCollection.InsertOne(context.Background(), &database.Task{
			UserID:       userID,
			SharedUntil:  futureTime,
			SharedAccess: &publicSharedAccess,
		})
		assert.NoError(t, err)
		taskID := mongoResult.InsertedID.(primitive.ObjectID)

		subtasksTitle := "subtasksTitle"
		taskCollection.InsertOne(context.Background(), &database.Task{
			UserID:       userID,
			ParentTaskID: taskID,
			Title:        &subtasksTitle,
		})

		response := ServeRequest(t, authToken, "GET", fmt.Sprintf("/shareable_tasks/detail/%s/", taskID.Hex()), nil, 200, api)
		var result ShareableTaskDetailsResponse
		err := json.Unmarshal(response, &result)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(result.Subtasks))
		assert.Equal(t, subtasksTitle, result.Subtasks[0].Title)
	})
	t.Run("CorrectDomainResponse", func(t *testing.T) {
		response := ServeRequest(t, authToken, "GET", fmt.Sprintf("/shareable_tasks/detail/%s/", domainSharedTaskID), nil, 200, api)
		var result ShareableTaskDetailsResponse
		err := json.Unmarshal(response, &result)
		assert.NoError(t, err)
		assert.Equal(t, "@resonant-kelpie-404a42.netlify.app", result.Domain)
	})
}
