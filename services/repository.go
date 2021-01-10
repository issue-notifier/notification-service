package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

var layout1 string = "2006-01-02T15:04:05-07:00"
var layout2 string = "2006-01-02 15:04:05-07:00"

type lastEventAtStruct struct {
	LastEventAt time.Time `json:"lastEventAt" db:"last_event_at"`
}

// Repository struct to store repository information from database
type Repository struct {
	RepoID      uuid.UUID `json:"repoID" db:"repo_id"`
	RepoName    string    `json:"repoName" db:"repo_name"`
	LastEventAt time.Time `json:"lastEventAt" db:"last_event_at"`
}

// Label struct to store label information from database
type Label struct {
	Name         string `json:"name" db:"label_name"`
	Color        string `json:"color" db:"label_color"`
	IsOfInterest bool   `json:"isOfInterest" db:"is_of_interest"`
}

// GetTextColor returns font text color based on label background color
func (l Label) GetTextColor() string {
	color := l.Color[1:]
	r, _ := strconv.ParseInt(color[0:2], 16, 32) // hexToR
	g, _ := strconv.ParseInt(color[2:4], 16, 32) // hexToG
	b, _ := strconv.ParseInt(color[4:6], 16, 32) // hexToB

	val := ((float64(r) * 0.299) + (float64(g) * 0.587) + (float64(b) * 0.114))

	if val > 186 {
		return "black"
	}

	return "white"
}

// GetAllRepositories gets all repositories via HTTP call to GET `/api/v1/repositories`
func GetAllRepositories() ([]Repository, error) {
	httpClient := &http.Client{}
	req, _ := http.NewRequest("GET", "http://localhost:8001/api/v1/repositories", nil)

	res, err := httpClient.Do(req)

	if err != nil {
		return nil, fmt.Errorf("[GetAllRepositories]: %v", err)
	}
	defer res.Body.Close()

	dataBytes, _ := ioutil.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Received %v from issue-notifier-api service %v", res.Status, string(dataBytes))
	}

	var data []map[string]interface{}
	json.Unmarshal(dataBytes, &data)

	var repositories []Repository
	for _, r := range data {
		repoID, _ := uuid.Parse(r["repoID"].(string))
		lastEventAt, _ := time.Parse(layout1, r["lastEventAt"].(string))
		repositories = append(repositories, Repository{
			RepoID:      repoID,
			RepoName:    r["repoName"].(string),
			LastEventAt: lastEventAt,
		})
	}

	return repositories, nil
}

// UpdateLastEventAt updates `lastEventAt` time for the given `repoID` via HTTP call to PUT `/api/v1/repository/{repoID}/update/lastEventAt`
func UpdateLastEventAt(repoID uuid.UUID, lastEventAt time.Time) error {
	reqBody, _ := json.Marshal(lastEventAtStruct{
		LastEventAt: lastEventAt,
	})

	httpClient := &http.Client{}
	req, _ := http.NewRequest("PUT", "http://localhost:8001/api/v1/repository/"+repoID.String()+"/update/lastEventAt", bytes.NewBuffer(reqBody))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	res, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("[UpdateLastEventAt]: %v", err)
	}
	defer res.Body.Close()

	dataBytes, _ := ioutil.ReadAll(res.Body)
	if res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("Received %v from issue-notifier-api service %v", res.Status, string(dataBytes))
	}

	var updateResponse string
	json.Unmarshal(dataBytes, &updateResponse)
	return nil
}
