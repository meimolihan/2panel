package service

import "github.com/google/uuid"

func newTaskID() string {
	return uuid.NewString()
}
