package api

import (
	"github.com/franchizzle/task-manager/backend/database"
	"github.com/franchizzle/task-manager/backend/external"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (api *API) TaskAddComment(c *gin.Context) {
	taskIDHex := c.Param("task_id")
	taskID, err := primitive.ObjectIDFromHex(taskIDHex)
	if err != nil {
		// This means the task ID is improperly formatted
		Handle404(c)
		return
	}

	userID := getUserIDFromContext(c)

	task, err := database.GetTask(api.DB, taskID, userID)
	if err != nil {
		c.JSON(404, gin.H{"detail": "task not found.", "taskId": taskID})
		return
	}

	var commentParams database.Comment
	err = c.BindJSON(&commentParams)
	if err != nil {
		c.JSON(400, gin.H{"detail": "parameter missing or malformatted"})
		return
	}
	// check if all fields are empty
	if commentParams == (database.Comment{}) {
		c.JSON(400, gin.H{"detail": "parameter missing"})
		return
	}

	taskSourceResult, err := api.ExternalConfig.GetSourceResult(task.SourceID)
	if err != nil {
		api.Logger.Error().Err(err).Msg("failed to load external task source")
		Handle500(c)
		return
	}

	if task.SourceID == external.TASK_SOURCE_ID_LINEAR {
		commentParams.ExternalID = uuid.New().String()
	}

	err = taskSourceResult.Source.AddComment(api.DB, userID, task.SourceAccountID, commentParams, task)
	if err != nil {
		api.Logger.Error().Err(err).Msg("failed to update external task source")
		Handle500(c)
		return
	}

	commentsPointer := task.Comments
	comments := []database.Comment{}
	if commentsPointer != nil {
		comments = *task.Comments
	}
	comments = append(comments, commentParams)

	updateTask := database.Task{
		Comments: &comments,
	}
	api.UpdateTaskInDB(c, task, userID, &updateTask)
	c.JSON(200, gin.H{})
}
