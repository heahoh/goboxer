package internal

const (
	statusToProcess = "new"
	statusDone      = "done"
	statusError     = "error"
)

type OutboxMessage struct {
	ID           int64  `field:"id"`
	Body         string `field:"body"`
	Meta         string `field:"meta"`
	Status       string `field:"status"`
	AttemptCount int8   `field:"attempt_count"`
}
