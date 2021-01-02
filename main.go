package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/issue-notifier/notification-service/database"
	"github.com/issue-notifier/notification-service/models"
	"github.com/issue-notifier/notification-service/services"
	"github.com/joho/godotenv"
)

// Env vars
var (
	DB_USER string
	DB_PASS string
	DB_NAME string

	GMAIL_ID       string
	GMAIL_PASSWORD string
)

var LAYOUT string = "2006-01-02T15:04:05-07:00"
var LAYOUT_2 string = "2006-01-02T15:04:05Z"
var BASE_TIME time.Time

type Repository struct {
	RepoName string
	Issues   []models.Issue
}

func main() {
	BASE_TIME, _ = time.Parse(LAYOUT, "1970-01-01T05:30:00+05:30")

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	DB_USER = os.Getenv("DB_USER")
	DB_PASS = os.Getenv("DB_PASS")
	DB_NAME = os.Getenv("DB_NAME")
	GMAIL_ID = os.Getenv("GMAIL_ID")
	GMAIL_PASSWORD = os.Getenv("GMAIL_PASSWORD")

	database.Init(DB_USER, DB_PASS, DB_NAME)
	defer database.DB.Close()

	repositories, err := services.GetAllRepositories()
	// If no repository found with the given name return an empty response
	if err == sql.ErrNoRows {
		return
	}

	for _, repository := range repositories {
		log.Println(repository.RepoName)
		// go processIssueEvents(repository)
	}

	sendEmails()

	// time.Sleep(1 * time.Hour)
}

func processIssueEvents(repository services.Repository) {
	subscriptionsByRepoID, _ := services.GetSubscriptionsByRepoID(repository.RepoID)

	userLabelSet := make(map[string]map[uuid.UUID]bool, len(subscriptionsByRepoID))
	// Used to store list of users who are interested for this label
	usersPerLabelMap := make(map[string][]uuid.UUID, len(subscriptionsByRepoID))
	// Used to store list of issues which contain this particular label
	issuesPerLabelMap := make(map[string][]float64, len(subscriptionsByRepoID))

	for _, sl := range subscriptionsByRepoID {
		labelName := sl["label"].(string)
		userID, _ := uuid.Parse(sl["userId"].(string))

		if _, exists := usersPerLabelMap[labelName]; exists {
			usersPerLabelMap[labelName] = append(usersPerLabelMap[labelName], userID)
		} else {
			usersPerLabelMap[labelName] = []uuid.UUID{userID}
			userLabelSet[labelName] = make(map[uuid.UUID]bool)
		}

		userLabelSet[labelName][userID] = true
	}

	var fetchEventsTill time.Time
	if repository.LastEventAt.Equal(BASE_TIME) {
		fetchEventsTill = time.Now().AddDate(0, 0, -1)
	} else {
		fetchEventsTill = repository.LastEventAt
	}
	log.Println("fetchEventsTill for repoName", repository.RepoName, fetchEventsTill)

	var events []map[string]interface{}
	httpClient := &http.Client{}
	pageNumber := 1
	for {
		// TODO: Think of Authorization as it can be a blocker once no. of repositories increases
		req, _ := http.NewRequest("GET", "https://api.github.com/repos/"+repository.RepoName+"/issues/events?page="+strconv.Itoa(pageNumber)+"&per_page=100&access_token=c078510e9604deb036e50bfa7599b30cd2f65a1f", nil)
		res, err := httpClient.Do(req)

		if err != nil {
			log.Fatalln(err)
		}

		defer res.Body.Close()

		dataBytes, _ := ioutil.ReadAll(res.Body)
		var data []map[string]interface{}

		json.Unmarshal(dataBytes, &data)
		events = append(events, data...)
		latestEventTime, _ := time.Parse(LAYOUT_2, data[len(data)-1]["created_at"].(string))
		log.Println("latestEventTime for repoName", repository.RepoName, latestEventTime)

		if latestEventTime.Before(fetchEventsTill) {
			break
		}

		pageNumber++
	}

	issues := make(map[float64]models.Issue, len(events))
	var newLatestEventTime time.Time
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]

		eventTime, _ := time.Parse(LAYOUT_2, e["created_at"].(string))
		if eventTime.Before(fetchEventsTill) {
			continue
		}

		newLatestEventTime = eventTime

		eventType := e["event"].(string)
		if eventType == "labeled" {
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
					State:          e["issue"].(map[string]interface{})["state"].(string),
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

	log.Println("newLatestEventTime for", repository.RepoName, "is:", newLatestEventTime)

	// If no issue events of interest found then return
	if len(issues) == 0 {
		services.UpdateLastEventAt(repository.RepoID, newLatestEventTime)
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

	err := models.CreateBulkNotificationsByRepoID(repository.RepoID, issueDataPerUserMap)
	if err == nil {
		services.UpdateLastEventAt(repository.RepoID, newLatestEventTime)
	} else {
		log.Println("Error occurred:", err, " while saving notification data for repository:", repository.RepoName)
	}
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

func sendEmails() {
	// Sender data.
	from := GMAIL_ID
	password := GMAIL_PASSWORD

	// Receiver email address.
	to := []string{
		"hsachdev.smart@gmail.com",
	}

	// smtp server configuration.
	smtpHost := "smtp.gmail.com"
	smtpPort := "587"

	issue1 := []models.Issue{
		{
			Title:          "Make fast-refresh compatible with ReScript",
			State:          "open",
			AssigneesCount: 0,
			Labels: []services.Label{
				{
					Name:         "pkgsite",
					Color:        "#ba4120",
					IsOfInterest: true,
				},
				{
					Name:         "WaitingForInfo",
					Color:        "#f09b1d",
					IsOfInterest: false,
				},
				{
					Name:         "FrozenDueToAge",
					Color:        "#fffb00",
					IsOfInterest: true,
				},
			},
			Number:    123456,
			CreatedAt: "sometime",
			UpdatedAt: "sometime2",
		},
		{
			Title:          "os/user: `panic: user: unknown userid 501` when call user.LookupId(\"501\") on Mac OS where 501 is the UserID of the admin",
			State:          "closed",
			AssigneesCount: 3,
			Labels: []services.Label{
				{
					Name:         "Proposal",
					Color:        "#08751a",
					IsOfInterest: true,
				},
				{
					Name:         "CLA Signed",
					Color:        "#084475",
					IsOfInterest: false,
				},
			},
			Number:    123456,
			CreatedAt: "sometime",
			UpdatedAt: "sometime2",
		},
	}

	data := struct {
		Username     string
		Repositories []Repository
	}{
		Username: "hemakshis",
		Repositories: []Repository{
			{
				RepoName: "facebook/react",
				Issues:   issue1,
			},
			{
				RepoName: "golang/go",
				Issues:   issue1,
			},
		},
	}

	log.Println(data)

	// Authentication.
	auth := smtp.PlainAuth("", from, password, smtpHost)

	t, err := template.ParseFiles("./email_templates/new_labeled_events.html")

	log.Println(err)

	var body bytes.Buffer

	mimeHeaders := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	body.Write([]byte(fmt.Sprintf("Subject: This is a test subject \n%s\n\n", mimeHeaders)))

	t.Execute(&body, data)

	// Sending email.
	err = smtp.SendMail(smtpHost+":"+smtpPort, auth, from, to, body.Bytes())
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Email Sent!")
}
