package database

import (
	"database/sql"
	"fmt"

	"github.com/issue-notifier/notification-service/utils"

	// Postgres driver for sql
	_ "github.com/lib/pq"
)

// DB to initialize postgres database
var DB *sql.DB

// Init initializes postgres database
func Init(dbUser, dbPass, dbName string) {
	connectionString := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable", dbUser, dbPass, dbName)
	var err error
	DB, err = sql.Open("postgres", connectionString)
	if err != nil {
		utils.LogError.Fatalln("Failed to connect to the database. Error:", err)
	}
	utils.LogInfo.Println("Successfully connected to the database")
}
