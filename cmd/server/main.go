package main

import (
	"fmt"
	"os"

	"github.com/seeman512/learn-pub-sub-starter/internal/gamelogic"
	"github.com/seeman512/learn-pub-sub-starter/internal/pubsub"
	"github.com/seeman512/learn-pub-sub-starter/internal/routing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	connectionString := "amqp://guest:guest@localhost:5672/"
	conn, err := amqp.Dial(connectionString)

	if err != nil {
		fmt.Printf("Connection error: %s\n", err)
		os.Exit(1)
	}

	fmt.Println("Connected successfuly")
	defer func() {
		fmt.Println("Connection closed")
		conn.Close()
	}()

	ch, err := conn.Channel()
	if err != nil {
		fmt.Printf("Channer error: %v\n", err)
		os.Exit(1)
	}

	// Log
	pubsub.SubscribeGOB(conn, string(routing.ExchangePerilTopic), "game_logs",
		routing.GameLogSlug+".*", pubsub.Durable, func(log routing.GameLog) pubsub.AckType {
			defer fmt.Sprintf("Log Message: %s\n", log.Message)
			gamelogic.WriteLog(log, false)
			return pubsub.Ack
		})

	gamelogic.PrintServerHelp()

	commands := map[string]func() bool{
		"pause": func() bool {
			fmt.Println("pause game")
			state := routing.PlayingState{IsPaused: true}
			err = pubsub.PublishJSON(ch, string(routing.ExchangePerilDirect), string(routing.PauseKey), state)

			if err != nil {
				fmt.Printf("publish error: %v\n", err)
			}

			return false
		},
		"resume": func() bool {
			fmt.Println("resume game")
			state := routing.PlayingState{IsPaused: false}
			err = pubsub.PublishJSON(ch, string(routing.ExchangePerilDirect), string(routing.PauseKey), state)

			if err != nil {
				fmt.Printf("publish error: %v\n", err)
			}

			return false
		},
		"help": func() bool {
			gamelogic.PrintServerHelp()
			return false
		},
		"quit": func() bool {
			fmt.Println("Exit")
			return true
		},
	}

	for {
		parts := gamelogic.GetInput()
		if len(parts) == 0 {
			continue
		}

		command := parts[0]
		action, ok := commands[command]

		if !ok {
			fmt.Println("Uknown command")
			continue
		}

		isBreak := action()
		if isBreak {
			break
		}
	}
}
