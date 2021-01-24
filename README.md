# Notification Service

The core service behind issue-notifier.

Fetches issue events per repository, filters them based on user's interest, stores them in the database and finally delivers :incoming_envelope: those issues to your inbox :mailbox_with_mail:!

### Feature Sets (yet to be implemented)
- [ ] Fetch events in almost real time. Currently, it runs the job once everyday (can be configured) but want to continuously poll the GitHub APIs.

Raise issues if you think your feature is needed.

### To run the service locally
1. You need to have Go & PostgreSQL installed
2. Start the [issue-notifier-api](https://github.com/issue-notifier/issue-notifier-api) service
2. Setup env vars
3. Run `$ go run main.go` 

### Contribution
1. Keep checking the Issues tab.
2. Find & solve `TODO`s in the source code and raise a PR
3. You can write unit tests!

Reach out to [Hemakshi Sachdev](https://github.com/hemakshis) for any queries