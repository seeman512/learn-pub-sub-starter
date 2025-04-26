package main

import (
	"fmt"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/seeman512/learn-pub-sub-starter/internal/gamelogic"
	"github.com/seeman512/learn-pub-sub-starter/internal/pubsub"
	"github.com/seeman512/learn-pub-sub-starter/internal/routing"
)

func main() {
	fmt.Println("Starting Peril client...")

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

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		fmt.Printf("Welcome error: %v\n", err)
		os.Exit(1)
	}

	queueName := routing.PauseKey + "." + username
	_, _, err = pubsub.DeclareAndBind(conn, routing.ExchangePerilDirect, queueName,
		routing.PauseKey, pubsub.Transient)

	if err != nil {
		fmt.Printf("Declare and bind error %v\n", err)
		os.Exit(1)
	}

	gameState := gamelogic.NewGameState(username)
	logger := gamelogic.Logger{Username: username, Debug: true}

	commands := map[string]func(args []string) bool{
		"spawn": func(args []string) bool {
			fmt.Printf("ARGS: %v\n", args)
			err := gameState.CommandSpawn(args)
			if err != nil {
				logger.Write(fmt.Sprintf("Spawn error: %v", err))
			}
			return false
		},
		"move": func(args []string) bool {
			_, err := gameState.CommandMove(args)
			if err != nil {
				logger.Write(fmt.Sprintf("Spawn error: %v", err))
			}
			return false
		},
		"status": func(args []string) bool {
			gameState.CommandStatus()
			return false
		},
		"help": func(args []string) bool {
			gamelogic.PrintClientHelp()
			return false
		},
		"spam": func(args []string) bool {
			logger.Write("Spamming not allowed yet")
			return false
		},
		"quit": func(args []string) bool {
			gamelogic.PrintQuit()
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

		isBreak := action(parts[:])
		if isBreak {
			break
		}
	}

}
