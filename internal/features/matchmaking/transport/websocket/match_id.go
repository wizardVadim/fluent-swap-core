package websocket

import (
	"github.com/google/uuid"
)

func GenerateMatchID() string {
	return uuid.NewString()
}
