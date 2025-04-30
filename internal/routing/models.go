package routing

import "time"

type PlayingState struct {
	IsPaused bool
}

type Move struct {
	ID       int
	Location string
	Username string
}

type GameLog struct {
	CurrentTime time.Time
	Message     string
	Username    string
}
