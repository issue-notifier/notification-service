package models

import (
	"github.com/google/uuid"
	"github.com/issue-notifier/notification-service/database"
)

type User struct {
	UserID   uuid.UUID `json:"userID" db:"user_id"`
	Username string    `json:"username" db:"username"`
	Email    string    `json:"email" db:"email"`
}

func GetAllUsersWithPendingNotificationData() ([]User, error) {
	sqlQuery := `SELECT DISTINCT GU.USER_ID, GU.USERNAME, GU.EMAIL
		FROM GITHUB_USER GU 
		INNER JOIN NOTIFICATION_DATA ND ON GU.USER_ID = ND.USER_ID 
		WHERE ND.SENT = 'F'`

	rows, err := database.DB.Query(sqlQuery)

	var data []User

	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var userID uuid.UUID
		var username, email string
		if err := rows.Scan(&userID, &username, &email); err != nil {
			return nil, err
		}

		data = append(data, User{
			UserID:   userID,
			Username: username,
			Email:    email,
		})
	}

	return data, nil
}
