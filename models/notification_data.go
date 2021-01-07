package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/issue-notifier/notification-service/database"
	"github.com/issue-notifier/notification-service/services"
)

// Issue struct defines basic data each issue holds
type Issue struct {
	Title          string           `json:"title" db:"title"`
	Number         float64          `json:"number" db:"number"`
	State          string           `json:"state" db:"state"`
	Labels         []services.Label `json:"labels" db:"labels"`
	CreatedAt      string           `json:"createdAt" db:"created_at"`
	UpdatedAt      string           `json:"updatedAt" db:"updated_at"`
	AssigneesCount int              `json:"assigneesCount" db:"assignees_count"`
}

// Value for the Issue struct to implement the driver Valuer interface. This method
// enables types to convert themselves to a driver Value.
func (a Issue) Value() (driver.Value, error) {
	return json.Marshal(a)
}

// Scan for the Issue struct implement the sql.Scanner interface. This method
// simply decodes a JSON-encoded value into the struct fields.
func (a *Issue) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(b, &a)
}

// GetAllPendingNotificationDataByUserID returns a []Issue and Repository information per repository for the given userID
func GetAllPendingNotificationDataByUserID(userID uuid.UUID) (map[string]interface{}, error) {
	sqlQuery := `SELECT GR.REPO_ID, GR.REPO_NAME, GR.LAST_EVENT_AT, ND.ISSUE_DATA 
		FROM NOTIFICATION_DATA ND 
		INNER JOIN GLOBAL_REPOSITORY GR ON GR.REPO_ID = ND.REPO_ID 
		WHERE ND.SENT = 'F' AND ND.USER_ID = $1`

	rows, err := database.DB.Query(sqlQuery, userID.String())
	if err != nil {
		return nil, fmt.Errorf("[GetAllPendingNotificationDataByUserID]: %v", err)
	}
	defer rows.Close()

	data := make(map[string]interface{})
	for rows.Next() {
		var repoID, repoName string
		var lastEventAt time.Time
		var issueData Issue
		if err := rows.Scan(&repoID, &repoName, &lastEventAt, &issueData); err != nil {
			return nil, fmt.Errorf("[GetAllPendingNotificationDataByUserID]: %v", err)
		}

		if _, exists := data[repoName]; !exists {
			data[repoName] = map[string]interface{}{
				"repoID":      repoID,
				"lastEventAt": lastEventAt,
				"issues":      []Issue{issueData},
			}
		} else {
			issueArr := data[repoName].(map[string]interface{})["issues"].([]Issue)
			data[repoName].(map[string]interface{})["issues"] = append(issueArr, issueData)
		}
	}

	return data, nil
}

// CreateBulkNotificationsByRepoID saves the given issueData (for each user) for the given repoID
func CreateBulkNotificationsByRepoID(repoID uuid.UUID, issueDataPerUserMap map[uuid.UUID]map[float64]Issue) error {
	sqlQuery := `INSERT INTO NOTIFICATION_DATA (USER_ID, REPO_ID, ISSUE_NUMBER, ISSUE_DATA) VALUES `

	valuesPlaceholder := make([]string, 0)
	values := make([]interface{}, 0)
	i := 0
	for userID, issues := range issueDataPerUserMap {
		for issueNumber, issueData := range issues {
			valuesPlaceholder = append(valuesPlaceholder, fmt.Sprintf("($%d, $%d, $%d, $%d)", i*4+1, i*4+2, i*4+3, i*4+4))
			values = append(values, userID)
			values = append(values, repoID)
			values = append(values, issueNumber)
			values = append(values, issueData)

			i++
		}
	}

	sqlQuery = sqlQuery + strings.Join(valuesPlaceholder, ",") + ` ON CONFLICT (REPO_ID, USER_ID, ISSUE_NUMBER) DO UPDATE SET ISSUE_DATA = EXCLUDED.ISSUE_DATA;`
	_, err := database.DB.Exec(sqlQuery, values...)
	if err != nil {
		return fmt.Errorf("[CreateBulkNotificationsByRepoID]: %v", err)
	}

	return nil
}

// UpdateSentNotificationData updates the status of `sent` to `true` for all notification data for the given userID and repoID
func UpdateSentNotificationData(userID, repoID string) error {
	sqlQuery := `UPDATE NOTIFICATION_DATA SET SENT = 'T' WHERE USER_ID = $1 AND REPO_ID = $2`

	_, err := database.DB.Exec(sqlQuery, userID, repoID)
	if err != nil {
		return fmt.Errorf("[UpdateSentNotificationData]: %v", err)
	}

	return nil
}

// DeleteAllSentNotificationData deletes all notification data where `sent` status = `true`
func DeleteAllSentNotificationData() error {
	sqlQuery := `DELETE FROM NOTIFICATION_DATA WHERE SENT = 'T'`

	_, err := database.DB.Exec(sqlQuery)
	if err != nil {
		return fmt.Errorf("[DeleteAllSentNotificationData]: %v", err)
	}

	return nil
}
