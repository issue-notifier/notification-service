package services

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/google/uuid"
)

func GetSubscriptionsByRepoID(repoID uuid.UUID) ([]map[string]interface{}, error) {
	httpClient := &http.Client{}
	req, _ := http.NewRequest("GET", "http://localhost:8001/api/v1/subscription/"+repoID.String()+"/view", nil)

	res, err := httpClient.Do(req)

	if err != nil {
		log.Fatalln(err)
	} else {
		defer res.Body.Close()

		dataBytes, _ := ioutil.ReadAll(res.Body)

		var data []map[string]interface{}
		json.Unmarshal(dataBytes, &data)

		return data, nil
	}

	return nil, err
}
