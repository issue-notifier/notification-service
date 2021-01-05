package services

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

type Repository struct {
	RepoID      uuid.UUID `json:"repoID" db:"repo_id"`
	RepoName    string    `json:"repoName" db:"repo_name"`
	LastEventAt time.Time `json:"lastEventAt" db:"last_event_at"`
}

type Label struct {
	Name         string `json:"name" db:"label_name"`
	Color        string `json:"color" db:"label_color"`
	IsOfInterest bool   `json:"isOfInterest" db:"is_of_interest"`
}

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
			lastEventAt, _ := time.Parse(LAYOUT, r["lastEventAt"].(string))
			repositories = append(repositories, Repository{
				RepoID:      repoID,
				RepoName:    r["repoName"].(string),
				LastEventAt: lastEventAt,
			})
		}

		return repositories, nil
	}

	return nil, err
}

func UpdateLastEventAt(repoID uuid.UUID, lastEventAt time.Time) error {
	reqBody, _ := json.Marshal(map[string]string{
		"lastEventAt": lastEventAt.Format("2006-01-02 15:04:05-07:00"),
	})

	httpClient := &http.Client{}
	req, _ := http.NewRequest("PUT", "http://localhost:8001/api/v1/repository/"+repoID.String()+"/update/lastEventAt", bytes.NewBuffer(reqBody))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	res, err := httpClient.Do(req)

	if err != nil {
		return err
	} else {
		defer res.Body.Close()
		data, _ := ioutil.ReadAll(res.Body)

		var updateResponse string
		json.Unmarshal(data, &updateResponse)

		log.Println(updateResponse)
	}

	return nil
}
