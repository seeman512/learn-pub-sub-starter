package gamelogic

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/seeman512/learn-pub-sub-starter/internal/routing"
)

const logsFile = "game.log"

const writeToDiskSleep = 400 * time.Millisecond

type Logger struct {
	Username string
	Debug    bool
}

func (l *Logger) Write(msg string) error {
	return WriteLog(routing.GameLog{
		CurrentTime: time.Now(),
		Message:     msg,
		Username:    l.Username,
	}, l.Debug)
}

func WriteLog(gamelog routing.GameLog, debug bool) error {
	log.Printf("received game log...")
	time.Sleep(writeToDiskSleep)

	f, err := os.OpenFile(logsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("could not open logs file: %v", err)
	}
	defer f.Close()

	str := fmt.Sprintf("%v %v: %v\n", gamelog.CurrentTime.Format(time.RFC3339), gamelog.Username, gamelog.Message)

	if debug {
		fmt.Println(str)
	}

	_, err = f.WriteString(str)
	if err != nil {
		return fmt.Errorf("could not write to logs file: %v", err)
	}
	return nil
}
