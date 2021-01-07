package models

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/issue-notifier/notification-service/database"
)

// User struct to store user information from database
type User struct {
	UserID   uuid.UUID `json:"userID" db:"user_id"`
	Username string    `json:"username" db:"username"`
	Email    string    `json:"email" db:"email"`
}

// GetAllUsersWithPendingNotificationData gets all distinct users who have pending notification data to be sent
func GetAllUsersWithPendingNotificationData() ([]User, error) {
	sqlQuery := `SELECT DISTINCT GU.USER_ID, GU.USERNAME, GU.EMAIL
		FROM GITHUB_USER GU 
		INNER JOIN NOTIFICATION_DATA ND ON GU.USER_ID = ND.USER_ID 
		WHERE ND.SENT = 'F'`

	rows, err := database.DB.Query(sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("[GetAllUsersWithPendingNotificationData]: %v", err)
	}
	defer rows.Close()

	var data []User
	for rows.Next() {
		var userID uuid.UUID
		var username, email string
		if err := rows.Scan(&userID, &username, &email); err != nil {
			return nil, fmt.Errorf("[GetAllUsersWithPendingNotificationData]: %v", err)
		}

		data = append(data, User{
			UserID:   userID,
			Username: username,
			Email:    email,
		})
	}

	return data, nil
}
