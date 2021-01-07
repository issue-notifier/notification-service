package services

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/google/uuid"
)

// GetSubscriptionsByRepoID gets all subscribed labels and the userID of the user who has subscribed for that particular for the give repoID
func GetSubscriptionsByRepoID(repoID uuid.UUID) ([]map[string]interface{}, error) {
	httpClient := &http.Client{}
	req, _ := http.NewRequest("GET", "http://localhost:8001/api/v1/subscription/"+repoID.String()+"/view", nil)

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("[GetSubscriptionsByRepoID] %v", err)
	}
	defer res.Body.Close()

	dataBytes, _ := ioutil.ReadAll(res.Body)

	var data []map[string]interface{}
	json.Unmarshal(dataBytes, &data)

	return data, nil
}
