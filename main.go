package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/issue-notifier/notification-service/database"
	"github.com/issue-notifier/notification-service/models"
	"github.com/issue-notifier/notification-service/services"
	"github.com/issue-notifier/notification-service/utils"
	"github.com/joho/godotenv"
)

// Env and global variables
var (
	dbUser string
	dbPass string
	dbName string

	gmailID   string
	gmailPass string

	tickerTime int64 // in hours
	timeGap    int64 // in minutes

	Layout1  string = "2006-01-02T15:04:05-07:00"
	Layout2  string = "2006-01-02T15:04:05Z"
	Layout3  string = "Jan 02, 2006 15:04"
	BaseTime time.Time
)

type repositoryData struct {
	RepoName    string
	LastEventAt string
	Issues      []models.Issue
}

func main() {
	utils.InitLogging()

	BaseTime, _ = time.Parse(Layout1, "1970-01-01T05:30:00+05:30")

	err := godotenv.Load()
	if err != nil {
		utils.LogError.Fatalln("Error loading .env file. Error:", err)
	}

	dbUser = os.Getenv("DB_USER")
	dbPass = os.Getenv("DB_PASS")
	dbName = os.Getenv("DB_NAME")
	gmailID = os.Getenv("GMAIL_ID")
	gmailPass = os.Getenv("GMAIL_PASSWORD")
	tickerTime, _ = strconv.ParseInt(os.Getenv("TICKER_TIME"), 10, 32)
	timeGap, _ = strconv.ParseInt(os.Getenv("TIME_GAP"), 10, 32)

	database.Init(dbUser, dbPass, dbName)
	defer database.DB.Close()

	ticker := time.NewTicker(time.Duration(tickerTime) * time.Hour)

	for range ticker.C {
		utils.LogInfo.Println("Starting to grab issue events per repository")
		start()
	}
}

func start() {
	repositories, err := services.GetAllRepositories()
	if err != nil {
		utils.LogError.Println("Failed to get all repositories. Error:", err)
		return
	}
	utils.LogInfo.Println("Got", len(repositories), "repositories")

	for _, repository := range repositories {
		go processIssueEvents(repository)
	}

	time.Sleep(time.Duration(timeGap) * time.Minute)

	users, err := models.GetAllUsersWithPendingNotificationData()
	if err != nil {
		utils.LogError.Println("Failed to get all users with pending notification data. Error:", err)
		return
	}
	utils.LogInfo.Println("Got", len(users), "users with pending notification data")

	for _, user := range users {
		go sendEmail(user)
	}

	time.Sleep(time.Duration(timeGap) * time.Minute)

	err = models.DeleteAllSentNotificationData()
	if err != nil {
		utils.LogError.Println("Failed to deleted all notification data with `sent` status equal to `true`. Error:", err)
		return
	}
	utils.LogInfo.Println("Successfully deleted all notification data with `sent` status equal to `true`")
}

func processIssueEvents(repository services.Repository) {
	utils.LogInfo.Println("Processing issue events for repository:", repository.RepoName)

	subscriptionsByRepoID, err := services.GetSubscriptionsByRepoID(repository.RepoID)
	if err != nil {
		utils.LogError.Println("Failed to get subscriptions for repository:", repository.RepoName, ". Error:", err)
		return
	}
	utils.LogInfo.Println("Got", len(subscriptionsByRepoID), "subscriptions for repository:", repository.RepoName)

	// Used to map users per label to get their interest
	userLabelSet := make(map[string]map[uuid.UUID]bool, len(subscriptionsByRepoID))
	// Used to store list of users who are interested for this label
	usersPerLabelMap := make(map[string][]uuid.UUID, len(subscriptionsByRepoID))
	// Used to store list of issues which contain this particular label
	issuesPerLabelMap := make(map[string][]float64, len(subscriptionsByRepoID))

	for _, sl := range subscriptionsByRepoID {
		labelName := sl["label"].(string)
		userID, _ := uuid.Parse(sl["userID"].(string))

		if _, exists := usersPerLabelMap[labelName]; exists {
			usersPerLabelMap[labelName] = append(usersPerLabelMap[labelName], userID)
		} else {
			usersPerLabelMap[labelName] = []uuid.UUID{userID}
			userLabelSet[labelName] = make(map[uuid.UUID]bool)
		}

		userLabelSet[labelName][userID] = true
	}

	var fetchEventsTill time.Time
	if repository.LastEventAt.Equal(BaseTime) {
		fetchEventsTill = time.Now().AddDate(0, 0, -1)
	} else {
		fetchEventsTill = repository.LastEventAt
	}
	utils.LogInfo.Println("Fetch events till:", fetchEventsTill, "for repository:", repository.RepoName)

	var events []map[string]interface{}
	httpClient := &http.Client{}
	pageNumber := 1
	var oldestEventTime time.Time
	var mostRecentEventTime time.Time
	for {
		// TODO: Think of Authorization as it can be a blocker once no. of repositories increases or use Etag maybe?
		req, _ := http.NewRequest("GET", "https://api.github.com/repos/"+repository.RepoName+"/issues/events?page="+strconv.Itoa(pageNumber)+"&per_page=100", nil)
		res, err := httpClient.Do(req)

		if err != nil {
			utils.LogError.Println("Failed to fetch issue events for repository:", repository.RepoName, "from GitHub. Error:", err)
		}

		defer res.Body.Close()

		dataBytes, _ := ioutil.ReadAll(res.Body)
		var data []map[string]interface{}

		json.Unmarshal(dataBytes, &data)
		events = append(events, data...)
		oldestEventTime, _ = time.Parse(Layout2, data[len(data)-1]["created_at"].(string))

		if pageNumber == 1 {
			mostRecentEventTime, _ = time.Parse(Layout2, data[0]["created_at"].(string))
		}

		if oldestEventTime.Before(fetchEventsTill) {
			break
		}

		pageNumber++
	}
	utils.LogInfo.Println("Paginated up to:", pageNumber, "pages. Fetched events from:", oldestEventTime, "to:", mostRecentEventTime, "for repository:", repository.RepoName)

	issues := make(map[float64]models.Issue, len(events))
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]

		eventTime, _ := time.Parse(Layout2, e["created_at"].(string))
		if eventTime.Before(fetchEventsTill) {
			continue
		}

		eventType := e["event"].(string)
		issueState := e["issue"].(map[string]interface{})["state"].(string)
		if eventType == "labeled" && issueState != "closed" {
			labelName := e["label"].(map[string]interface{})["name"].(string)
			if _, isLabelOfInterest := usersPerLabelMap[labelName]; isLabelOfInterest {
				labelsObject := e["issue"].(map[string]interface{})["labels"].([]interface{})
				var labels []services.Label
				for _, l := range labelsObject {
					label := services.Label{
						Name:  l.(map[string]interface{})["name"].(string),
						Color: "#" + l.(map[string]interface{})["color"].(string),
					}

					labels = append(labels, label)
				}

				issueNumber := e["issue"].(map[string]interface{})["number"].(float64)
				issues[issueNumber] = models.Issue{
					Number:         issueNumber,
					Title:          e["issue"].(map[string]interface{})["title"].(string),
					State:          issueState,
					Labels:         labels,
					CreatedAt:      e["issue"].(map[string]interface{})["created_at"].(string),
					UpdatedAt:      e["issue"].(map[string]interface{})["updated_at"].(string),
					AssigneesCount: len(e["issue"].(map[string]interface{})["assignees"].([]interface{})),
				}

				if _, exists := issuesPerLabelMap[labelName]; exists {
					issuesPerLabelMap[labelName] = append(issuesPerLabelMap[labelName], issueNumber)
				} else {
					issuesPerLabelMap[labelName] = []float64{issueNumber}
				}

			}
		}
	}
	utils.LogInfo.Println("Got", len(issues), "issue events for repository:", repository.RepoName)

	// If no issue events of interest found then return
	if len(issues) == 0 {
		err := services.UpdateLastEventAt(repository.RepoID, mostRecentEventTime)
		if err != nil {
			utils.LogError.Println("Failed to update `lastEventAt` time for repository:", repository.RepoName, ". Error:", err)
			return
		}
		utils.LogInfo.Println("Updated `lastEventAt` time to:", mostRecentEventTime, "for repository:", repository.RepoName)
		return
	}

	issuesPerUserMap := make(map[uuid.UUID][]float64, len(issues))
	for labelName, users := range usersPerLabelMap {
		if len(issuesPerLabelMap[labelName]) > 0 {
			for _, user := range users {
				if _, exists := issuesPerUserMap[user]; exists {

					issuesPerUserMap[user] = append(issuesPerUserMap[user], issuesPerLabelMap[labelName]...)
				} else {
					issuesPerUserMap[user] = issuesPerLabelMap[labelName]
				}
			}
		}
	}

	issueDataPerUserMap := make(map[uuid.UUID]map[float64]models.Issue, len(issues))
	for userID, userIssues := range issuesPerUserMap {
		issueDataPerUserMap[userID] = getIssuesWithData(userID, userIssues, issues, userLabelSet)
	}

	err = models.CreateBulkNotificationsByRepoID(repository.RepoID, issueDataPerUserMap)
	if err != nil {
		utils.LogError.Println("Failed to save notification data for repository:", repository.RepoName, ". Error:", err)
		return
	}

	err = services.UpdateLastEventAt(repository.RepoID, mostRecentEventTime)
	if err != nil {
		utils.LogError.Println("Failed to update `lastEventAt` time for repository:", repository.RepoName, ". Error:", err)
	}
	utils.LogInfo.Println("Updated `lastEventAt` time to:", mostRecentEventTime, "for repository:", repository.RepoName)
}

func getIssuesWithData(userID uuid.UUID, userIssues []float64, issues map[float64]models.Issue, userLabelSet map[string]map[uuid.UUID]bool) map[float64]models.Issue {
	data := make(map[float64]models.Issue, len(userIssues))
	for _, ui := range userIssues {
		if _, exists := data[ui]; !exists {

			issueData := issues[ui]
			for li, la := range issueData.Labels {
				issueData.Labels[li].IsOfInterest = userLabelSet[la.Name][userID]
			}

			data[ui] = issueData
		}
	}

	return data
}

func sendEmail(user models.User) {
	// smtp server configuration.
	smtpHost := "smtp.gmail.com"
	smtpPort := "587"

	issuesPerRepositoryMap, err := models.GetAllPendingNotificationDataByUserID(user.UserID)
	if err != nil {
		utils.LogError.Println("Failed to get notification data for user:", user.UserID, ". Error:", err)
		return
	}

	var repositories []repositoryData
	for repoName, repoData := range issuesPerRepositoryMap {
		lastEventAt := repoData.(map[string]interface{})["lastEventAt"].(time.Time).Format(Layout3)
		issueDataArr := repoData.(map[string]interface{})["issues"].([]models.Issue)
		repositories = append(repositories, repositoryData{
			RepoName:    repoName,
			LastEventAt: lastEventAt,
			Issues:      issueDataArr,
		})
		utils.LogInfo.Println("Got", len(issueDataArr), "issues for repository:", repoName)
	}

	data := struct {
		Username     string
		Repositories []repositoryData
	}{
		Username:     user.Username,
		Repositories: repositories,
	}

	// Authentication.
	auth := smtp.PlainAuth("", gmailID, gmailPass, smtpHost)

	templateFilePath := "./email_templates/new_labeled_events.html"
	t, err := template.ParseFiles(templateFilePath)
	if err != nil {
		utils.LogError.Println("Failed to parse template file:", templateFilePath, ". Error:", err)
		return
	}

	var body bytes.Buffer
	mimeHeaders := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	body.Write([]byte(fmt.Sprintf("Subject: New issues awaiting to be resolved, go get 'em! \n%s\n\n", mimeHeaders)))

	t.Execute(&body, data)

	// Sending email.
	err = smtp.SendMail(smtpHost+":"+smtpPort, auth, gmailID, []string{user.Email}, body.Bytes())
	if err != nil {
		utils.LogError.Println("Failed to send email to user:", user.UserID, ". Error:", err)
		return
	}
	utils.LogInfo.Println("Successfully sent email to user:", user.UserID)

	for repoName, repoData := range issuesPerRepositoryMap {
		err := models.UpdateSentNotificationData(user.UserID.String(), repoData.(map[string]interface{})["repoID"].(string))
		if err != nil {
			utils.LogError.Println("Failed to update `sent` status to `true` for all notification data for user:", user.UserID, " and repository:", repoName, ". Error:", err)
			continue
		}
		utils.LogInfo.Println("Updated `sent` status to `true` for all notification data for user:", user.UserID, " and repository:", repoName)
	}

}
