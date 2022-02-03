package api

import (
	"context"
	"log"
	"time"

	"github.com/GeneralTask/task-manager/backend/constants"
	"github.com/GeneralTask/task-manager/backend/database"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type FeedbackParams struct {
	Feedback string `json:"feedback"`
}

func (api *API) FeedbackAdd(c *gin.Context) {
	parentCtx := c.Request.Context()
	var params FeedbackParams
	err := c.BindJSON(&params)
	if err != nil || params.Feedback == "" {
		log.Printf("error: %v", err)
		c.JSON(400, gin.H{"detail": "invalid or missing 'feedback' parameter."})
		return
	}

	db, dbCleanup, err := database.GetDBConnection()
	if err != nil {
		Handle500(c)
		return
	}
	defer dbCleanup()
	feedbackCollection := database.GetFeedbackItemCollection(db)

	userID, _ := c.Get("user")

	dbCtx, cancel := context.WithTimeout(parentCtx, constants.DatabaseTimeout)
	defer cancel()
	_, err = feedbackCollection.InsertOne(
		dbCtx,
		&database.FeedbackItem{
			UserID:    userID.(primitive.ObjectID),
			Feedback:  params.Feedback,
			CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		},
	)
	if err != nil {
		log.Printf("failed to insert feedback item: %v", err)
		Handle500(c)
		return
	}
	c.JSON(201, gin.H{})
}
