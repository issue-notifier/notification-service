package services

// IssueNotifierAPIEndpoint endpoint for the API service, comes from .env file
var (
	IssueNotifierAPIEndpoint string
)

// Init initializes the IssueNotifierAPIEndpoint endpoint from the .env file
func Init(issueNotifierAPIEndpoint string) {
	IssueNotifierAPIEndpoint = issueNotifierAPIEndpoint
}
