package repo

import (
	"time"

	"github.com/google/uuid"
)

func newTaskID() string {
	return uuid.NewString()
}

func now() time.Time {
	return time.Now()
}
