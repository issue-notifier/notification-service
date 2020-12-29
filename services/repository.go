package services

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type Repository struct {
	RepoID             uuid.UUID `json:"repoID" db:"repo_id"`
	RepoName           string    `json:"repoName" db:"repo_name"`
	LastEventFetchedAt time.Time `json:"lastEventFetchedAt" db:"last_event_fetched_at"`
}

type Label struct {
	Name         string `json:"name" db:"label_name"`
	Color        string `json:"color" db:"label_color"`
	IsOfInterest bool   `json:"isOfInterest" db:"is_of_interest"`
}

var LAYOUT string = "2006-01-02T15:04:05-07:00"

func GetAllRepositories() ([]Repository, error) {
	httpClient := &http.Client{}
	req, _ := http.NewRequest("GET", "http://localhost:8001/api/v1/repositories", nil)

	res, err := httpClient.Do(req)

	if err != nil {
		log.Fatalln(err)
	} else {
		defer res.Body.Close()

		dataBytes, _ := ioutil.ReadAll(res.Body)

		var data []map[string]interface{}
		json.Unmarshal(dataBytes, &data)

		var repositories []Repository
		for _, r := range data {
			repoID, _ := uuid.Parse(r["repoID"].(string))
			lastEventFetchedAt, _ := time.Parse(LAYOUT, r["lastEventFetchedAt"].(string))
			repositories = append(repositories, Repository{
				RepoID:             repoID,
				RepoName:           r["repoName"].(string),
				LastEventFetchedAt: lastEventFetchedAt,
			})
		}

		return repositories, nil
	}

	return nil, err
}
