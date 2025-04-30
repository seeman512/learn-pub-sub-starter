package main

import (
	"fmt"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/seeman512/learn-pub-sub-starter/internal/gamelogic"
	"github.com/seeman512/learn-pub-sub-starter/internal/pubsub"
	"github.com/seeman512/learn-pub-sub-starter/internal/routing"
)

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	return func(playState routing.PlayingState) pubsub.AckType {
		defer fmt.Printf("> ")
		gs.Paused = playState.IsPaused

		return pubsub.Ack
	}
}

func handleMove(gs *gamelogic.GameState, ch *amqp.Channel) func(gamelogic.ArmyMove) pubsub.AckType {
	return func(move gamelogic.ArmyMove) pubsub.AckType {
		fmt.Printf("Handle Move: %v \n", move)
		defer fmt.Printf("> ")

		outcome := gs.HandleMove(move)

		switch outcome {
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeMakeWar:
			topicKey := routing.WarRecognitionsPrefix + "." + gs.GetUsername()
			rw := gamelogic.RecognitionOfWar{
				Attacker: move.Player,
				Defender: gs.Player,
			}
			err := pubsub.PublishJSON(ch, string(routing.ExchangePerilTopic), topicKey, rw)
			if err != nil {
				fmt.Printf("Publish move -> war error: %v; requeue\n", err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.NackDiscard
		default:
			return pubsub.NackDiscard
		}
	}
}

func handleWar(gs *gamelogic.GameState, ch *amqp.Channel) func(gamelogic.RecognitionOfWar) pubsub.AckType {
	return func(rw gamelogic.RecognitionOfWar) pubsub.AckType {
		fmt.Printf("Handle War: %v \n", rw)
		defer fmt.Printf("> ")

		userName := gs.Player.Username
		opponentName := rw.Attacker.Username

		warOutcome, _, _ := gs.HandleWar(rw)
		log := routing.GameLog{
			CurrentTime: time.Now(),
			Message:     "",
			Username:    gs.Player.Username,
		}

		switch warOutcome {
		case gamelogic.WarOutcomeNotInvolved:
			// returnVal = pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			// returnVal = pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon:
			// returnVal = pubsub.Ack
			log.Message = fmt.Sprintf("%s won a war against %s", opponentName, userName)
		case gamelogic.WarOutcomeYouWon:
			// returnVal = pubsub.Ack
			log.Message = fmt.Sprintf("%s won a war against %s", userName, opponentName)
		case gamelogic.WarOutcomeDraw:
			// returnVal = pubsub.Ack
			log.Message = fmt.Sprintf("A war between %s and %s resulted in a draw", userName, opponentName)
		default:
			fmt.Printf("Wrong outcome: %v\n", warOutcome)
			//returnVal = pubsub.NackDiscard
		}

		topicKey := routing.GameLogSlug + "." + opponentName
		err := pubsub.PublishGOB(ch, string(routing.ExchangePerilTopic), topicKey, log)
		if err != nil {
			fmt.Printf("Publish log error: %v; requeue\n", err)
			return pubsub.NackRequeue
		}
		return pubsub.Ack

	}
}

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

	ch, err := conn.Channel()
	if err != nil {
		fmt.Printf("Channer error: %v\n", err)
		os.Exit(1)
	}

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

	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilDirect, queueName,
		routing.PauseKey, pubsub.Transient, handlerPause(gameState))

	if err != nil {
		fmt.Printf("Subscribe error %v\n", err)
		os.Exit(1)
	}

	// Topic exchange
	// Move
	moveTopicQueue := routing.ArmyMovesPrefix + "." + username
	moveTopicKey := string(routing.ArmyMovesPrefix) + ".*"
	err = pubsub.SubscribeJSON(conn, string(routing.ExchangePerilTopic), moveTopicQueue,
		moveTopicKey, pubsub.Transient, handleMove(gameState, ch))
	if err != nil {
		fmt.Printf("Subscribe %s error %v\n", moveTopicQueue, err)
		os.Exit(1)
	}
	//War
	warTopicKey := string(routing.WarRecognitionsPrefix) + ".*"
	err = pubsub.SubscribeJSON(conn, string(routing.ExchangePerilTopic), "war",
		warTopicKey, pubsub.Durable, handleWar(gameState, ch))
	if err != nil {
		fmt.Printf("Subscribe war error %v\n", err)
		os.Exit(1)
	}
	//

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

			// TODO Publish ArmyMove
			units := []gamelogic.Unit{}

			for u := range args[1:] {
				unit, ok := gameState.GetUnit(int(u))
				if ok {
					units = append(units, unit)
				}
			}

			move := gamelogic.ArmyMove{
				Player:     gameState.Player,
				Units:      units,
				ToLocation: gamelogic.Location(args[0]),
			}
			pubsub.PublishJSON(ch, string(routing.ExchangePerilTopic), moveTopicKey, move)
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
