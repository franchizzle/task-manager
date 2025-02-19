package api

import (
	"context"

	"golang.org/x/exp/slices"

	"github.com/franchizzle/task-manager/backend/constants"
	"github.com/franchizzle/task-manager/backend/database"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
)

type CalendarResult struct {
	CalendarID      string `json:"calendar_id,omitempty"`
	ColorID         string `json:"color_id,omitempty"`
	Title           string `json:"title,omitempty"`
	CanWrite        bool   `json:"can_write,omitempty"`
	AccessRole      string `json:"access_role,omitempty"`
	ColorBackground string `json:"color_background,omitempty"`
	ColorForeground string `json:"color_foreground,omitempty"`
}

type CalendarAccountResult struct {
	AccountID               string           `json:"account_id"`
	Calendars               []CalendarResult `json:"calendars"`
	HasMulticalScope        bool             `json:"has_multical_scopes"`
	HasPrimaryCalendarScope bool             `json:"has_primary_calendar_scopes"`
}

func (api *API) CalendarsList(c *gin.Context) {
	userID := getUserIDFromContext(c)
	var userObject database.User
	userCollection := database.GetUserCollection(api.DB)
	err := userCollection.FindOne(context.Background(), bson.M{"_id": userID}).Decode(&userObject)
	if err != nil {
		api.Logger.Error().Err(err).Msg("failed to find user")
		Handle500(c)
		return
	}

	calendarAccounts, err := database.GetCalendarAccounts(api.DB, userID)
	if err != nil {
		Handle500(c)
		return
	}
	results := []*CalendarAccountResult{}
	for _, account := range *calendarAccounts {
		// for implicit memory aliasing
		calendars := []CalendarResult{}
		for _, calendar := range account.Calendars {
			calendarResult := CalendarResult{
				CalendarID: calendar.CalendarID,
				ColorID:    calendar.ColorID,
				Title:      calendar.Title,
				CanWrite:   slices.Contains([]string{constants.AccessControlOwner, "writer"}, calendar.AccessRole),
				AccessRole: calendar.AccessRole,
        ColorBackground: calendar.ColorBackground,
				ColorForeground: calendar.ColorForeground,
			}
			calendars = append(calendars, calendarResult)

		}
		result := CalendarAccountResult{
			AccountID:               account.IDExternal,
			Calendars:               calendars,
			HasMulticalScope:        database.HasUserGrantedMultiCalendarScope(account.Scopes),
			HasPrimaryCalendarScope: database.HasUserGrantedPrimaryCalendarScope(account.Scopes),
		}
		results = append(results, &result)
	}

	c.JSON(200, results)
}
