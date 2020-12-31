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

func (a Issue) Value() (driver.Value, error) {
	return json.Marshal(a)
}

// Make the Label struct implement the sql.Scanner interface. This method
// simply decodes a JSON-encoded value into the struct fields.
func (a *Issue) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(b, &a)
}

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

	return err
}
