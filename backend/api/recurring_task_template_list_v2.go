package api

import (
	"github.com/franchizzle/task-manager/backend/database"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (api *API) RecurringTaskTemplateListV2(c *gin.Context) {
	userID := getUserIDFromContext(c)

	var templates []database.RecurringTaskTemplate
	opts := options.Find().SetSort(bson.M{"created_at": -1})
	err := database.FindWithCollection(database.GetRecurringTaskTemplateCollection(api.DB), userID, nil, &templates, opts)
	if err != nil {
		api.Logger.Error().Err(err).Msg("failed to fetch recurring task templates")
		Handle500(c)
		return
	}

	c.JSON(200, templates)
}
