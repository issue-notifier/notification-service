package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/issue-notifier/notification-service/database"
	"github.com/issue-notifier/notification-service/services"
)

type Issue struct {
	Title          string           `json:"title" db:"title"`
	Number         float64          `json:"number" db:"number"`
	State          string           `json:"state" db:"state"`
	Labels         []services.Label `json:"labels" db:"labels"`
	CreatedAt      string           `json:"createdAt" db:"created_at"`
	UpdatedAt      string           `json:"updatedAt" db:"updated_at"`
	AssigneesCount int              `json:"assigneesCount" db:"assignees_count"`
}

type Issues []Issue

func (a Issues) Value() (driver.Value, error) {
	return json.Marshal(a)
}

// Make the Label struct implement the sql.Scanner interface. This method
// simply decodes a JSON-encoded value into the struct fields.
func (a *Issues) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(b, &a)
}

func CreateBulkNotificationsByRepoID(repoID uuid.UUID, issueDataPerUserMap map[uuid.UUID]map[float64]Issue) error {
	sqlQuery := `INSERT INTO NOTIFICATION_DATA (USER_ID, REPO_ID, ISSUES) VALUES `

	valuesPlaceholder := make([]string, 0)
	values := make([]interface{}, 0)
	i := 0
	for userID, issueData := range issueDataPerUserMap {
		valuesPlaceholder = append(valuesPlaceholder, fmt.Sprintf("($%d, $%d, $%d)", i*3+1, i*3+2, i*3+3))
		values = append(values, userID)
		values = append(values, repoID)
		values = append(values, getIssueDataAsSlice(issueData))

		i++
	}

	sqlQuery = sqlQuery + strings.Join(valuesPlaceholder, ",")
	_, err := database.DB.Exec(sqlQuery, values...)

	return err
}

func getIssueDataAsSlice(issueData map[float64]Issue) Issues {
	var issueDataSlice Issues

	for _, v := range issueData {
		issueDataSlice = append(issueDataSlice, v)
	}

	return issueDataSlice
}
