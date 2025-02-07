package api

import (
	"context"
	"testing"

	"github.com/franchizzle/task-manager/backend/database"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestGetValidExternalOwnerAssignedTask(t *testing.T) {
	api, dbCleanup := GetAPIWithDBCleanup()
	defer dbCleanup()

	userCollection := database.GetUserCollection(api.DB)
	julianUser, err := userCollection.InsertOne(context.Background(), database.User{
		Email: "julian@resonant-kelpie-404a42.netlify.app",
	})
	assert.NoError(t, err)
	johnUser, err := userCollection.InsertOne(context.Background(), database.User{
		Email: "john@resonant-kelpie-404a42.netlify.app",
	})
	assert.NoError(t, err)

	t.Run("InvalidCallingUser", func(t *testing.T) {
		_, title, err := getValidExternalOwnerAssignedTask(api.DB, primitive.NewObjectID(), "HELLO!")
		assert.Error(t, err)
		assert.Equal(t, "", title)
	})
	t.Run("InvalidDestinationUser", func(t *testing.T) {
		_, title, err := getValidExternalOwnerAssignedTask(api.DB, primitive.NewObjectID(), "<to example>HELLO!")
		assert.Error(t, err)
		assert.Equal(t, "", title)
	})
	t.Run("InvalidTitle", func(t *testing.T) {
		_, title, err := getValidExternalOwnerAssignedTask(api.DB, julianUser.InsertedID.(primitive.ObjectID), "HELLO!")
		assert.Error(t, err)
		assert.Equal(t, "", title)
	})
	t.Run("Success", func(t *testing.T) {
		user, title, err := getValidExternalOwnerAssignedTask(api.DB, julianUser.InsertedID.(primitive.ObjectID), "<to john>Hello there!")
		assert.NoError(t, err)
		assert.Equal(t, "Hello there! from: julian@resonant-kelpie-404a42.netlify.app", title)
		assert.Equal(t, johnUser.InsertedID.(primitive.ObjectID), user.ID)
	})
}
