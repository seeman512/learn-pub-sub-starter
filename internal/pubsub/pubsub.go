package pubsub

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type QueueType int

const Durable QueueType = 1
const Transient QueueType = 2

type AckType int

const (
	Ack         AckType = 1
	NackRequeue AckType = 2
	NackDiscard AckType = 3
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	valBytes, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("marshal error %w", err)
	}

	err = ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        valBytes,
	})

	if err != nil {
		return fmt.Errorf("publish error %w", err)
	}

	return nil
}

func PublishGOB[T any](ch *amqp.Channel, exchange, key string, val T) error {
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)

	err := encoder.Encode(val)
	if err != nil {
		return fmt.Errorf("marshal error %w", err)
	}

	err = ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{
		ContentType: "application/gob",
		Body:        buffer.Bytes(),
	})

	if err != nil {
		return fmt.Errorf("publish error %w", err)
	}

	return nil
}

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType QueueType, // an enum to represent "durable" or "transient"
	handler func(T) AckType,
) error {
	unmarshaller := func(data []byte) (T, error) {
		var obj T
		err := json.Unmarshal(data, &obj)
		return obj, err
	}

	return subscribe(conn, exchange, queueName, key, simpleQueueType, handler, unmarshaller)
}

func SubscribeGOB[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType QueueType, // an enum to represent "durable" or "transient"
	handler func(T) AckType,
) error {
	unmarshaller := func(data []byte) (T, error) {
		var obj T
		decoder := gob.NewDecoder(bytes.NewBuffer(data))
		err := decoder.Decode(&obj)
		return obj, err
	}

	return subscribe(conn, exchange, queueName, key, simpleQueueType, handler, unmarshaller)
}

func subscribe[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType QueueType, // an enum to represent "durable" or "transient"
	handler func(T) AckType,
	unmarshaller func([]byte) (T, error),
) error {
	ch, _, err := DeclareAndBind(conn, exchange, queueName, key, simpleQueueType)

	if err != nil {
		return err
	}

	deliveryCh, err := ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume channel error: %w", err)
	}

	go func() {
		for msg := range deliveryCh {
			var obj T
			// err := json.Unmarshal(msg.Body, &obj)
			obj, err := unmarshaller(msg.Body)
			if err != nil {
				fmt.Printf("Unmarshall error %v\n", err)
				continue
			}

			isValid := handler(obj)
			switch isValid {
			case Ack:
				msg.Ack(false)
				fmt.Println("Ack")
			case NackRequeue:
				msg.Nack(false, true)
				fmt.Println("Nack Requeue")
			case NackDiscard:
				msg.Nack(false, false)
				fmt.Println("Nack Discard")
			default:
				fmt.Println("Nack Discard; wrong")
				msg.Nack(false, false)
			}
		}
	}()

	return nil
}

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType QueueType, // an enum to represent "durable" or "transient"
) (*amqp.Channel, amqp.Queue, error) {
	ch, err := conn.Channel()
	var queue amqp.Queue

	// transient flags
	durable := false
	autoDelete := true
	exclusive := true

	noWait := false

	if simpleQueueType == Durable {
		durable = true
		autoDelete = false
		exclusive = false
	}

	if err != nil {
		return nil, queue, err
	}

	args := map[string]any{
		"x-dead-letter-exchange": "peril_dlx",
	}
	queue, err = ch.QueueDeclare(queueName, durable, autoDelete, exclusive, noWait, amqp.Table(args))
	if err != nil {
		return nil, queue, err
	}

	err = ch.QueueBind(queueName, key, exchange, false, nil)
	if err != nil {
		return nil, queue, err
	}

	return ch, queue, nil
}
