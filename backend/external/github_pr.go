package external

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/franchizzle/task-manager/backend/logging"
	"golang.org/x/oauth2"

	"github.com/franchizzle/task-manager/backend/constants"
	"github.com/franchizzle/task-manager/backend/database"
	"github.com/google/go-github/v45/github"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	CurrentlyAuthedUserFilter string = ""
	RepoOwnerTypeOrganization string = "Organization"
	StateApproved             string = "APPROVED"
	StateChangesRequested     string = "CHANGES_REQUESTED"
	StateCommented            string = "COMMENTED"
)

// *Important*: Add all required actions to the ActionOrdering map so that the PRs are ordered correctly
// *Also important*: Update PULL_REQUEST_REQUIRED_ACTIONS on the frontend if you add a new action
// And also please keep these sorted based on priority
const (
	ActionReviewPR          string = "Review PR"
	ActionAddReviewers      string = "Add Reviewers"
	ActionFixFailedCI       string = "Fix Failed CI"
	ActionAddressComments   string = "Address Comments"
	ActionFixMergeConflicts string = "Fix Merge Conflicts"
	ActionWaitingOnCI       string = "Waiting on CI"
	ActionMergePR           string = "Merge PR"
	ActionWaitingOnReview   string = "Waiting on Review"
	ActionWaitingOnAuthor   string = "Waiting on Author"
	ActionNoneNeeded        string = "Not Actionable"
)

var ActionOrdering = map[string]int{
	ActionReviewPR:          0,
	ActionAddReviewers:      1,
	ActionFixFailedCI:       2,
	ActionAddressComments:   3,
	ActionFixMergeConflicts: 4,
	ActionWaitingOnCI:       5,
	ActionMergePR:           6,
	ActionWaitingOnReview:   7,
	ActionWaitingOnAuthor:   8,
	ActionNoneNeeded:        9,
}

const (
	ChecksStatusCompleted    string = "completed"
	ChecksConclusionFailure  string = "failure"
	ChecksConclusionTimedOut string = "timed_out"
)

const (
	GithubAPIBaseURL string = "https://api.github.com/"
)

type GithubPRSource struct {
	Github GithubService
}

type GithubPRData struct {
	RequestedReviewers   int
	Reviewers            *github.Reviewers
	IsMergeable          bool
	IsApproved           bool
	HaveRequestedChanges bool
	ChecksDidFail        bool
	ChecksDidFinish      bool
	IsOwnedByUser        bool
	UserLogin            string
	UserIsReviewer       bool
}

type GithubPRRequestData struct {
	Client      *github.Client
	User        *github.User
	Repository  *github.Repository
	PullRequest *github.PullRequest
	Token       *oauth2.Token
	UserTeams   []*github.Team
}

type GithubUserResult struct {
	User  *github.User
	Error error
}

type GithubUserTeamsResult struct {
	UserTeams []*github.Team
	Error     error
}

type GithubRepositoriesResult struct {
	Repositories []*github.Repository
	Error        error
}

type ProcessRepositoryResult struct {
	PullRequestChannels []chan *database.PullRequest
	RequestTimes        []primitive.DateTime
	Error               error
	ShouldLog           bool
}

func (gitPR GithubPRSource) GetEvents(db *mongo.Database, userID primitive.ObjectID, accountID string, startTime time.Time, endTime time.Time, scopes []string, result chan<- CalendarResult) {
	result <- emptyCalendarResult(errors.New("github PR cannot fetch events"))
}

func (gitPR GithubPRSource) GetTasks(db *mongo.Database, userID primitive.ObjectID, accountID string, result chan<- TaskResult) {
	result <- emptyTaskResult(nil)
}

func (gitPR GithubPRSource) GetPullRequests(db *mongo.Database, userID primitive.ObjectID, accountID string, result chan<- PullRequestResult) {
	logger := logging.GetSentryLogger()
	err := database.InsertLogEvent(db, userID, "get_pull_requests")
	if err != nil {
		logger.Error().Err(err).Msg("error inserting log event")
	}
	parentCtx := context.Background()

	var githubClient *github.Client
	// need to copy github client for each async call so that override url setting is threadsafe
	var githubClientUser *github.Client
	var githubClientTeams *github.Client
	var githubClientRepos *github.Client
	extCtx, cancel := context.WithTimeout(parentCtx, constants.ExternalTimeout)
	defer cancel()

	var token *oauth2.Token
	if gitPR.Github.Config.ConfigValues.FetchExternalAPIToken != nil && *gitPR.Github.Config.ConfigValues.FetchExternalAPIToken {
		externalAPITokenCollection := database.GetExternalTokenCollection(db)
		// need to do this to ensure `token` is the same `token` initialized above vs creating a new one with the := operator
		var err error
		token, err = GetGithubToken(externalAPITokenCollection, userID, accountID)
		if token == nil {
			logger.Error().Msg("failed to fetch Github API token")
			result <- emptyPullRequestResult(errors.New("failed to fetch Github API token"), false)
			return
		}
		if err != nil {
			result <- emptyPullRequestResult(err, false)
			return
		}

		githubClient = getGithubClientFromToken(extCtx, token)
		githubClientUser = getGithubClientFromToken(extCtx, token)
		githubClientTeams = getGithubClientFromToken(extCtx, token)
		githubClientRepos = getGithubClientFromToken(extCtx, token)
	} else {
		githubClient = github.NewClient(nil)
		githubClientUser = github.NewClient(nil)
		githubClientTeams = github.NewClient(nil)
		githubClientRepos = github.NewClient(nil)
	}

	extCtx, cancel = context.WithTimeout(parentCtx, constants.ExternalTimeout)
	defer cancel()

	userResultChan := make(chan GithubUserResult)
	go getGithubUser(extCtx, githubClientUser, CurrentlyAuthedUserFilter, gitPR.Github.Config.ConfigValues.GetUserURL, userResultChan)

	userTeamsResultChan := make(chan GithubUserTeamsResult)
	go getUserTeams(extCtx, githubClientTeams, gitPR.Github.Config.ConfigValues.ListUserTeamsURL, userTeamsResultChan)

	repositoriesResultChan := make(chan GithubRepositoriesResult)
	go getGithubRepositories(extCtx, githubClientRepos, CurrentlyAuthedUserFilter, gitPR.Github.Config.ConfigValues.ListRepositoriesURL, repositoriesResultChan)

	userResult := <-userResultChan
	if userResult.Error != nil || userResult.User == nil {
		shouldLog := handleErrorLogging(userResult.Error, db, userID, "failed to fetch Github user")
		result <- emptyPullRequestResult(errors.New("failed to fetch Github user"), !shouldLog)
		return
	}

	userTeamsResult := <-userTeamsResultChan
	if userTeamsResult.Error != nil {
		shouldLog := handleErrorLogging(userTeamsResult.Error, db, userID, "failed to fetch Github user teams")
		result <- emptyPullRequestResult(errors.New("failed to fetch Github user teams"), !shouldLog)
		return
	}

	repositoriesResult := <-repositoriesResultChan
	if repositoriesResult.Error != nil {
		shouldLog := handleErrorLogging(repositoriesResult.Error, db, userID, "failed to fetch Github repos for user")
		result <- emptyPullRequestResult(errors.New("failed to fetch Github repos for user"), !shouldLog)
		return
	}

	processRepositoryResultChannels := []chan ProcessRepositoryResult{}
	for _, repository := range repositoriesResult.Repositories {
		processRepositoryResultChan := make(chan ProcessRepositoryResult)
		go gitPR.processRepository(db, userID, accountID, repository, githubClient, token, userResult.User, userTeamsResult.UserTeams, processRepositoryResultChan)
		processRepositoryResultChannels = append(processRepositoryResultChannels, processRepositoryResultChan)
	}

	var pullRequestChannels []chan *database.PullRequest
	var requestTimes []primitive.DateTime
	for _, processRepositoryResultChan := range processRepositoryResultChannels {
		processRepositoryResult := <-processRepositoryResultChan
		if processRepositoryResult.Error != nil {
			result <- emptyPullRequestResult(errors.New("failed to process Github repo"), !processRepositoryResult.ShouldLog)
		}
		pullRequestChannels = append(pullRequestChannels, processRepositoryResult.PullRequestChannels...)
		requestTimes = append(requestTimes, processRepositoryResult.RequestTimes...)
	}

	var pullRequests []*database.PullRequest
	for index, pullRequestChan := range pullRequestChannels {
		pullRequest := <-pullRequestChan
		// if nil, this means that the request ran into an error: continue and keep processing the rest
		if pullRequest == nil {
			continue
		}

		// don't update or create if it's a cached PR from the DB, unless needs to be marked incomplete
		if pullRequest.ID != primitive.NilObjectID && (pullRequest.IsCompleted == nil || !*pullRequest.IsCompleted) {
			pullRequests = append(pullRequests, pullRequest)
			continue
		}

		isCompleted := false
		pullRequest.IsCompleted = &isCompleted
		pullRequest.LastFetched = requestTimes[index]
		dbPR, err := database.UpdateOrCreatePullRequest(
			db,
			userID,
			string(pullRequest.IDExternal),
			pullRequest.SourceID,
			pullRequest,
			nil)
		if err != nil {
			logger.Error().Err(err).Msg("failed to update or create pull request")
			result <- emptyPullRequestResult(err, false)
			return
		}
		pullRequest.ID = dbPR.ID
		pullRequest.IDOrdering = dbPR.IDOrdering

		pullRequests = append(pullRequests, pullRequest)
	}

	result <- PullRequestResult{
		PullRequests: pullRequests,
		Error:        nil,
	}
}

func (gitPR GithubPRSource) processRepository(db *mongo.Database, userID primitive.ObjectID, accountID string, repository *github.Repository, githubClient *github.Client, token *oauth2.Token, githubUser *github.User, userTeams []*github.Team, result chan<- ProcessRepositoryResult) {
	err := updateOrCreateRepository(db, repository, accountID, userID)
	if err != nil {
		logging.GetSentryLogger().Error().Err(err).Msg("failed to update or create repository")
		result <- ProcessRepositoryResult{Error: err}
		return
	}
	extCtx, cancel := context.WithTimeout(context.Background(), constants.ExternalTimeout)
	defer cancel()
	fetchedPullRequests, err := getGithubPullRequests(extCtx, githubClient, repository, gitPR.Github.Config.ConfigValues.ListPullRequestsURL)
	if err != nil && shouldLogError(err) {
		shouldLog := handleErrorLogging(err, db, userID, "failed to fetch Github PRs")
		result <- ProcessRepositoryResult{Error: err, ShouldLog: shouldLog}
		return
	}
	err = database.InsertLogEvent(db, userID, "list_pull_requests")
	if err != nil {
		logging.GetSentryLogger().Error().Err(err).Msg("failed to insert log event")
	}
	var pullRequestChannels []chan *database.PullRequest
	var requestTimes []primitive.DateTime
	for _, pullRequest := range fetchedPullRequests {
		pullRequestChan := make(chan *database.PullRequest)
		requestData := GithubPRRequestData{
			Client:      githubClient,
			User:        githubUser,
			Repository:  repository,
			PullRequest: pullRequest,
			Token:       token,
			UserTeams:   userTeams,
		}
		requestTimes = append(requestTimes, primitive.NewDateTimeFromTime(time.Now()))
		go gitPR.getPullRequestInfo(db, userID, accountID, requestData, pullRequestChan)
		pullRequestChannels = append(pullRequestChannels, pullRequestChan)
	}
	result <- ProcessRepositoryResult{PullRequestChannels: pullRequestChannels, RequestTimes: requestTimes}
}

func (gitPR GithubPRSource) getPullRequestInfo(db *mongo.Database, userID primitive.ObjectID, accountID string, requestData GithubPRRequestData, result chan<- *database.PullRequest) {
	err := database.InsertLogEvent(db, userID, "get_pull_request_info")
	if err != nil {
		logging.GetSentryLogger().Error().Err(err).Msg("failed to insert log event")
	}
	githubClient := requestData.Client
	githubUser := requestData.User
	repository := requestData.Repository
	pullRequest := requestData.PullRequest

	// do the check
	extCtx, cancel := context.WithTimeout(context.Background(), constants.ExternalTimeout)
	defer cancel()
	hasBeenModified, cachedPR := pullRequestHasBeenModified(db, extCtx, userID, requestData, gitPR.Github.Config.ConfigValues.PullRequestModifiedURL)
	if !hasBeenModified {
		result <- cachedPR
		return
	}

	err = setOverrideURL(githubClient, gitPR.Github.Config.ConfigValues.ListPullRequestReviewURL)
	if err != nil {
		handleErrorLogging(err, db, userID, "failed to set override url for Github PR reviews")
		result <- nil
		return
	}
	reviews, _, err := githubClient.PullRequests.ListReviews(extCtx, *repository.Owner.Login, *repository.Name, *pullRequest.Number, nil)
	if err != nil {
		handleErrorLogging(err, db, userID, "failed to fetch Github PR reviews")
		result <- nil
		return
	}

	// refresh context to prevent timeout
	extCtx, cancel = context.WithTimeout(context.Background(), constants.ExternalTimeout)
	defer cancel()
	comments, err := getComments(extCtx, githubClient, repository, pullRequest, reviews, gitPR.Github.Config.ConfigValues.ListPullRequestCommentsURL, gitPR.Github.Config.ConfigValues.ListIssueCommentsURL)
	if err != nil {
		handleErrorLogging(err, db, userID, "failed to fetch Github PR comments")
		result <- nil
		return
	}

	additions, deletions, numCommits, err := getAdditionsDeletions(extCtx, githubClient, repository, pullRequest, gitPR.Github.Config.ConfigValues.CompareURL)
	// if the comparison isn't found, still show the PR but with blank additions / deletions
	// TODO: have frontend hide the additions / deletions when zeroed out
	if err != nil && !strings.Contains(err.Error(), "404 Not Found") {
		handleErrorLogging(err, db, userID, "failed to fetch Github PR additions / deletions")
		result <- nil
		return
	}

	requiredAction := ActionNoneNeeded
	isOwner := userIsOwner(githubUser, pullRequest)
	if isOwner || userIsReviewer(githubUser, pullRequest, reviews, requestData.UserTeams) {
		extCtx, cancel = context.WithTimeout(context.Background(), constants.ExternalTimeout)
		defer cancel()

		reviewers, err := listReviewers(extCtx, githubClient, repository, pullRequest, gitPR.Github.Config.ConfigValues.ListPullRequestReviewersURL)
		if err != nil {
			handleErrorLogging(err, db, userID, "failed to fetch Github PR reviewers")
			result <- nil
			return
		}
		requestedReviewers, err := getReviewerCount(extCtx, githubClient, repository, pullRequest, reviews, gitPR.Github.Config.ConfigValues.ListPullRequestReviewersURL)
		if err != nil {
			handleErrorLogging(err, db, userID, "failed to fetch Github PR reviewers")
			result <- nil
			return
		}
		pullRequestFetch, _, err := githubClient.PullRequests.Get(extCtx, *repository.Owner.Login, *repository.Name, *pullRequest.Number)
		if err != nil {
			handleErrorLogging(err, db, userID, "failed to fetch Github PR")
			result <- nil
			return
		}
		// check runs are individual tests that make up a check suite associated with a commit
		checkRunsForCommit, err := listCheckRunsForCommit(extCtx, githubClient, repository, pullRequest, gitPR.Github.Config.ConfigValues.ListCheckRunsForRefURL)
		if err != nil {
			handleErrorLogging(err, db, userID, "failed to fetch Github PR check runs")
			result <- nil
			return
		}
		checksDidFail := checkRunsDidFail(checkRunsForCommit)
		checksDidFinish := checkRunsDidFinish(checkRunsForCommit)

		requiredAction = getPullRequestRequiredAction(GithubPRData{
			RequestedReviewers:   requestedReviewers,
			Reviewers:            reviewers,
			IsMergeable:          pullRequestFetch.GetMergeable(),
			IsApproved:           pullRequestIsApproved(reviews),
			HaveRequestedChanges: reviewersHaveRequestedChanges(reviews),
			ChecksDidFail:        checksDidFail,
			ChecksDidFinish:      checksDidFinish,
			IsOwnedByUser:        isOwner,
			UserLogin:            githubUser.GetLogin(),
			UserIsReviewer:       userNeedsToSubmitReview(githubUser, reviewers, requestData.UserTeams),
		})
	}

	result <- &database.PullRequest{
		UserID:            userID,
		IDExternal:        fmt.Sprint(pullRequest.GetID()),
		Deeplink:          pullRequest.GetHTMLURL(),
		SourceID:          TASK_SOURCE_ID_GITHUB_PR,
		Title:             pullRequest.GetTitle(),
		Body:              pullRequest.GetBody(),
		SourceAccountID:   accountID,
		CreatedAtExternal: primitive.NewDateTimeFromTime(pullRequest.GetCreatedAt()),
		RepositoryID:      fmt.Sprint(*repository.ID),
		RepositoryName:    repository.GetFullName(),
		Number:            pullRequest.GetNumber(),
		Author:            pullRequest.User.GetLogin(),
		Branch:            pullRequest.Head.GetRef(),
		BaseBranch:        pullRequest.Base.GetRef(),
		RequiredAction:    requiredAction,
		Comments:          comments,
		CommentCount:      len(comments),
		CommitCount:       numCommits,
		Additions:         additions,
		Deletions:         deletions,
		LastUpdatedAt:     primitive.NewDateTimeFromTime(pullRequest.GetUpdatedAt()),
	}
}

func handleErrorLogging(err error, db *mongo.Database, userID primitive.ObjectID, msg string) bool {
	shouldLog := shouldLogError(err)
	if shouldLog {
		logging.GetSentryLogger().Error().Err(err).Msg(msg)
	}
	if strings.Contains(err.Error(), "403 API rate limit") {
		err := database.InsertLogEvent(db, userID, "github_pr_rate_limited")
		if err != nil {
			logging.GetSentryLogger().Error().Err(err).Msg(msg)
		}
	}
	return shouldLog
}

func shouldLogError(err error) bool {
	errorString := err.Error()
	for _, errorSubstring := range []string{"404 Not Found", "451 Repository access blocked", "403 API rate limit"} {
		if strings.Contains(errorString, errorSubstring) {
			return false
		}
	}
	return true
}

func setOverrideURL(githubClient *github.Client, overrideURL *string) error {
	var err error
	var baseURL *url.URL
	if overrideURL != nil {
		baseURL, err = url.Parse(fmt.Sprintf("%s/", *overrideURL))
		*githubClient.BaseURL = *baseURL
	}
	return err
}

func pullRequestHasBeenModified(db *mongo.Database, ctx context.Context, userID primitive.ObjectID, requestData GithubPRRequestData, overrideURL *string) (bool, *database.PullRequest) {
	logger := logging.GetSentryLogger()

	pullRequest := requestData.PullRequest
	token := requestData.Token
	repository := requestData.Repository

	dbPR, err := database.GetPullRequestByExternalID(db, fmt.Sprint(*pullRequest.ID), userID)
	if err != nil {
		// if fail to fetch from DB, fetch from Github
		if err != mongo.ErrNoDocuments {
			logger.Error().Err(err).Msg("unable to fetch pull request from db")
		}
		return true, nil
	}

	requestURL := GithubAPIBaseURL + "repos/" + *repository.Owner.Login + "/" + *repository.Name + "/pulls/" + fmt.Sprint(*pullRequest.Number)
	if overrideURL != nil {
		requestURL = *overrideURL
	}

	// Github API does not support conditional requests, so this logic is required
	request, _ := http.NewRequest("GET", requestURL, nil)
	request.Header.Set("Accept", "application/vnd.github+json")
	if token != nil {
		request.Header.Set("Authorization", "token "+token.AccessToken)
	}
	if !dbPR.LastFetched.Time().IsZero() {
		request.Header.Set("If-Modified-Since", (dbPR.LastFetched.Time().Format("Mon, 02 Jan 2006 15:04:05 MST")))
	}
	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		logger.Error().Err(err).Msg("error with github http request")
		return true, dbPR
	}

	return (resp.StatusCode != http.StatusNotModified), dbPR
}

func getGithubUser(ctx context.Context, githubClient *github.Client, currentlyAuthedUserFilter string, overrideURL *string, result chan<- GithubUserResult) {
	err := setOverrideURL(githubClient, overrideURL)
	if err != nil {
		result <- GithubUserResult{Error: err}
		return
	}
	githubUser, _, err := githubClient.Users.Get(ctx, currentlyAuthedUserFilter)
	result <- GithubUserResult{User: githubUser, Error: err}
}

func getUserTeams(context context.Context, githubClient *github.Client, overrideURL *string, result chan<- GithubUserTeamsResult) {
	err := setOverrideURL(githubClient, overrideURL)
	if err != nil {
		result <- GithubUserTeamsResult{Error: err}
		return
	}
	userTeams, _, err := githubClient.Teams.ListUserTeams(context, nil)
	result <- GithubUserTeamsResult{UserTeams: userTeams, Error: err}
}

func getGithubRepositories(ctx context.Context, githubClient *github.Client, currentlyAuthedUserFilter string, overrideURL *string, result chan<- GithubRepositoriesResult) {
	err := setOverrideURL(githubClient, overrideURL)
	if err != nil {
		result <- GithubRepositoriesResult{Error: err}
	}
	// we sort by "pushed" to put the more active repos near the front of the results
	// 30 results are returned by default, which should be enough, but we can increase to 100 if needed
	repositoryListOptions := github.RepositoryListOptions{Sort: "pushed"}
	repositories, _, err := githubClient.Repositories.List(ctx, currentlyAuthedUserFilter, &repositoryListOptions)
	result <- GithubRepositoriesResult{Repositories: repositories, Error: err}
}

func updateOrCreateRepository(db *mongo.Database, repository *github.Repository, accountID string, userID primitive.ObjectID) error {
	repositoryCollection := database.GetRepositoryCollection(db)
	_, err := repositoryCollection.UpdateOne(
		context.Background(),
		bson.M{"$and": []bson.M{
			// TODO: add account_id to query once backfill is completed
			{"repository_id": fmt.Sprint(repository.GetID())},
			{"user_id": userID},
		}},
		bson.M{"$set": bson.M{
			"account_id": accountID,
			"full_name":  repository.GetFullName(),
			"deeplink":   repository.GetHTMLURL(),
		}},
		options.Update().SetUpsert(true),
	)
	return err
}

func getGithubPullRequests(ctx context.Context, githubClient *github.Client, repository *github.Repository, overrideURL *string) ([]*github.PullRequest, error) {
	err := setOverrideURL(githubClient, overrideURL)
	if err != nil {
		return nil, err
	}
	if repository == nil || repository.Owner == nil || repository.Owner.Login == nil {
		return nil, errors.New("repository is nil")
	}
	fetchedPullRequests, _, err := githubClient.PullRequests.List(ctx, *repository.Owner.Login, *repository.Name, nil)
	return fetchedPullRequests, err
}

func listReviewers(ctx context.Context, githubClient *github.Client, repository *github.Repository, pullRequest *github.PullRequest, overrideURL *string) (*github.Reviewers, error) {
	err := setOverrideURL(githubClient, overrideURL)
	if err != nil {
		return nil, err
	}
	if repository == nil || repository.Owner == nil || repository.Owner.Login == nil {
		return nil, errors.New("repository is nil")
	}
	if pullRequest == nil || pullRequest.Number == nil {
		return nil, errors.New("pull request is nil")
	}
	reviewers, _, err := githubClient.PullRequests.ListReviewers(ctx, *repository.Owner.Login, *repository.Name, *pullRequest.Number, nil)
	return reviewers, err
}

func listComments(context context.Context, githubClient *github.Client, repository *github.Repository, pullRequest *github.PullRequest, overrideURL *string) ([]*github.PullRequestComment, error) {
	err := setOverrideURL(githubClient, overrideURL)
	if err != nil {
		return nil, err
	}
	comments, _, err := githubClient.PullRequests.ListComments(context, *repository.Owner.Login, *repository.Name, *pullRequest.Number, nil)
	return comments, err
}

func listIssueComments(context context.Context, githubClient *github.Client, repository *github.Repository, pullRequest *github.PullRequest, overrideURL *string) ([]*github.IssueComment, error) {
	err := setOverrideURL(githubClient, overrideURL)
	if err != nil {
		return nil, err
	}
	issueComments, _, err := githubClient.Issues.ListComments(context, *repository.Owner.Login, *repository.Name, *pullRequest.Number, nil)
	return issueComments, err
}

func listCheckRunsForCommit(ctx context.Context, githubClient *github.Client, repository *github.Repository, pullRequest *github.PullRequest, overrideURL *string) (*github.ListCheckRunsResults, error) {
	err := setOverrideURL(githubClient, overrideURL)
	if err != nil {
		return nil, err
	}
	checkRuns, _, err := githubClient.Checks.ListCheckRunsForRef(ctx, *repository.Owner.Login, *repository.Name, *pullRequest.Head.SHA, nil)
	return checkRuns, err
}

func userIsOwner(githubUser *github.User, pullRequest *github.PullRequest) bool {
	return (githubUser.ID != nil &&
		pullRequest.User.ID != nil &&
		*githubUser.ID == *pullRequest.User.ID)
}

func userNeedsToSubmitReview(githubUser *github.User, reviewers *github.Reviewers, userTeams []*github.Team) bool {
	if githubUser == nil || reviewers == nil {
		return false
	}
	for _, reviewer := range reviewers.Users {
		if reviewer.GetID() == githubUser.GetID() {
			return true
		}
	}
	for _, userTeam := range userTeams {
		for _, team := range reviewers.Teams {
			if team.GetID() == userTeam.GetID() {
				return true
			}
		}
	}
	return false
}

// Github API does not consider users who have submitted a review as reviewers
func userIsReviewer(githubUser *github.User, pullRequest *github.PullRequest, reviews []*github.PullRequestReview, userTeams []*github.Team) bool {
	if pullRequest == nil || githubUser == nil {
		return false
	}
	for _, reviewer := range pullRequest.RequestedReviewers {
		if githubUser.ID != nil && reviewer.ID != nil && *githubUser.ID == *reviewer.ID {
			return true
		}
	}
	for _, userTeam := range userTeams {
		for _, team := range pullRequest.RequestedTeams {
			if team.ID != nil && userTeam.ID != nil && *team.ID == *userTeam.ID {
				return true
			}
		}
	}
	// If user submitted a review, we consider them a reviewer as well
	for _, review := range reviews {
		if githubUser.GetID() == review.User.GetID() {
			return true
		}
	}
	return false
}

func pullRequestIsApproved(pullRequestReviews []*github.PullRequestReview) bool {
	for _, review := range pullRequestReviews {
		if review.State != nil && *review.State == StateApproved {
			return true
		}
	}
	return false
}

func getComments(context context.Context, githubClient *github.Client, repository *github.Repository, pullRequest *github.PullRequest, reviews []*github.PullRequestReview, overrideURLPRComments *string, overrideURLIssueComments *string) ([]database.PullRequestComment, error) {
	if repository == nil {
		return nil, errors.New("repository is nil")
	}
	if pullRequest == nil {
		return nil, errors.New("pull request is nil")
	}
	result := []database.PullRequestComment{}
	comments, err := listComments(context, githubClient, repository, pullRequest, overrideURLPRComments)
	if err != nil {
		return nil, err
	}
	for _, comment := range comments {
		result = append(result, database.PullRequestComment{
			Type:            constants.COMMENT_TYPE_INLINE,
			Body:            comment.GetBody(),
			Author:          comment.User.GetLogin(),
			Filepath:        comment.GetPath(),
			LineNumberStart: comment.GetStartLine(),
			LineNumberEnd:   comment.GetLine(),
			CreatedAt:       primitive.NewDateTimeFromTime(comment.GetCreatedAt()),
		})
	}
	issueComments, err := listIssueComments(context, githubClient, repository, pullRequest, overrideURLIssueComments)
	if err != nil {
		return nil, err
	}
	for _, issueComment := range issueComments {
		result = append(result, database.PullRequestComment{
			Type:      constants.COMMENT_TYPE_TOPLEVEL,
			Body:      issueComment.GetBody(),
			Author:    issueComment.User.GetLogin(),
			CreatedAt: primitive.NewDateTimeFromTime(issueComment.GetCreatedAt()),
		})
	}
	for _, review := range reviews {
		body := review.GetBody()
		if body == "" {
			state := review.GetState()
			if state == StateApproved {
				body = "(Approved changes)"
			} else if state == StateChangesRequested {
				body = "(Requested changes)"
			} else {
				body = "(Reviewed changes)"
			}
		}
		result = append(result, database.PullRequestComment{
			Type:      constants.COMMENT_TYPE_TOPLEVEL,
			Body:      body,
			Author:    review.User.GetLogin(),
			CreatedAt: primitive.NewDateTimeFromTime(review.GetSubmittedAt()),
		})
	}
	return result, nil
}

func getAdditionsDeletions(context context.Context, githubClient *github.Client, repository *github.Repository, pullRequest *github.PullRequest, overrideURLCompare *string) (int, int, int, error) {
	err := setOverrideURL(githubClient, overrideURLCompare)
	if err != nil {
		return 0, 0, 0, err
	}
	comparison, _, err := githubClient.Repositories.CompareCommits(context, repository.Owner.GetLogin(), repository.GetName(), pullRequest.Base.GetRef(), pullRequest.Head.GetRef(), nil)
	if err != nil {
		return 0, 0, 0, err
	}
	additions := 0
	deletions := 0
	for _, file := range comparison.Files {
		additions += file.GetAdditions()
		deletions += file.GetDeletions()
	}
	return additions, deletions, comparison.GetTotalCommits(), nil
}

func getReviewerCount(context context.Context, githubClient *github.Client, repository *github.Repository, pullRequest *github.PullRequest, reviews []*github.PullRequestReview, overrideURL *string) (int, error) {
	if repository == nil {
		return 0, errors.New("repository is nil")
	}
	if pullRequest == nil {
		return 0, errors.New("pull request is nil")
	}
	reviewers, err := listReviewers(context, githubClient, repository, pullRequest, overrideURL)
	if err != nil {
		return 0, err
	}
	submittedReviews := 0
	for _, review := range reviews {
		state := review.GetState()
		if review.GetUser() != nil && (state == StateApproved || state == StateChangesRequested) {
			submittedReviews += 1
		}
	}
	return submittedReviews + len(reviewers.Users) + len(reviewers.Teams), nil
}

func reviewersHaveRequestedChanges(reviews []*github.PullRequestReview) bool {
	userToMostRecentReview := make(map[string]string)
	for _, review := range reviews {
		reviewState := review.GetState()
		// If a user requests changes, and then leaves a comment, the PR is still in the 'changes requested' state.
		if reviewState == StateCommented {
			continue
		}
		userToMostRecentReview[review.GetUser().GetLogin()] = reviewState
	}
	for _, review := range userToMostRecentReview {
		if review == StateChangesRequested {
			return true
		}
	}
	return false
}

func checkRunsDidFinish(checkRuns *github.ListCheckRunsResults) bool {
	for _, checkRun := range checkRuns.CheckRuns {
		if checkRun.GetStatus() != ChecksStatusCompleted {
			return false
		}
	}
	return true
}

func checkRunsDidFail(checkRuns *github.ListCheckRunsResults) bool {
	for _, run := range checkRuns.CheckRuns {
		if run.GetStatus() == ChecksStatusCompleted && (run.GetConclusion() == ChecksConclusionFailure || run.GetConclusion() == ChecksConclusionTimedOut) {
			return true
		}
	}
	return false
}

func getPullRequestRequiredAction(data GithubPRData) string {
	var action string
	if data.IsOwnedByUser {
		if data.RequestedReviewers == 0 {
			action = ActionAddReviewers
		} else if data.ChecksDidFail {
			action = ActionFixFailedCI
		} else if data.HaveRequestedChanges {
			action = ActionAddressComments
		} else if !data.IsMergeable {
			action = ActionFixMergeConflicts
		} else if !data.ChecksDidFinish {
			action = ActionWaitingOnCI
		} else if data.IsApproved {
			action = ActionMergePR
		} else {
			action = ActionWaitingOnReview
		}
	} else {
		if data.UserIsReviewer {
			action = ActionReviewPR
		}
		if action == "" {
			action = ActionWaitingOnAuthor
		}
	}
	return action
}

func (gitPR GithubPRSource) CreateNewTask(db *mongo.Database, userID primitive.ObjectID, accountID string, task TaskCreationObject) (primitive.ObjectID, error) {
	return primitive.NilObjectID, errors.New("has not been implemented yet")
}

func (gitPR GithubPRSource) CreateNewEvent(db *mongo.Database, userID primitive.ObjectID, accountID string, event EventCreateObject) error {
	return errors.New("has not been implemented yet")
}

func (gitPR GithubPRSource) DeleteEvent(db *mongo.Database, userID primitive.ObjectID, accountID string, externalID string, calendarID string) error {
	return errors.New("has not been implemented yet")
}

func (gitPR GithubPRSource) ModifyTask(db *mongo.Database, userID primitive.ObjectID, accountID string, issueID string, updateFields *database.Task, task *database.Task) error {
	// allow users to mark PR as done in GT even if it's not done in Github
	return nil
}

func (gitPR GithubPRSource) ModifyEvent(db *mongo.Database, userID primitive.ObjectID, accountID string, eventID string, updateFields *EventModifyObject) error {
	return errors.New("has not been implemented yet")
}

func (gitPR GithubPRSource) AddComment(db *mongo.Database, userID primitive.ObjectID, accountID string, comment database.Comment, task *database.Task) error {
	return errors.New("has not been implemented yet")
}
