package main

import (
	"encoding/json"
	"log"
	"net"
	"os"
	"os/exec"

	"github.com/streadway/amqp"
)

func getIpAddress() string {
	addrs, err := net.InterfaceAddrs()
	FailOnError(err, "Failed to get IP address")

	for _, address := range addrs {
		// Check the address type and make sure it is not loopback
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}

		}
	}
	return ""
}

func serveOperation(oper Operation, ch *amqp.Channel, q string) {
	var resp Response
	var value bool = false
	if oper.Type == "file_exists" {
		// Check if the file exists in the given path
		if _, err := os.Stat(oper.Path); err == nil {
			value = true
		}
	} else if oper.Type == "process_running" {
		// Check if the process is running using pgrep
		_, err := exec.Command("pgrep", oper.Process).Output()
		if err == nil {
			value = true
		}
	} else if oper.Type == "file_contains" {
		if _, err := os.Stat(oper.Path); err == nil {
			_, err := exec.Command("grep", oper.Check, oper.Path).Output()
			if err == nil {
				value = true
			}
		}
	}
	resp.Name = oper.Name
	resp.Value = value
	resp.Hostname, _ = os.Hostname()
	resp.Ip = getIpAddress()
	body, _ := json.Marshal(resp)
	err := ch.Publish(
		"",    // exchange
		q,     // routing key
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
	FailOnError(err, "Failed to respond to message")
	log.Println("Served query:", resp.Name)
}

func main() {
	// Connect to RabbitMQ
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	FailOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()
	log.Println("Connected to RabbitMQ")

	// Create a channel
	ch, err := conn.Channel()
	FailOnError(err, "Failed to open a channel")
	defer ch.Close()
	log.Println("Opened a channel")

	// Setup exchange
	err = ch.ExchangeDeclare(
		EXCHANGE_NAME, // name
		"fanout",      // type
		true,          // durable
		false,         // auto-deleted
		false,         // internal
		false,         // no-wait
		nil,           // arguments
	)
	FailOnError(err, "Failed to declare an exchange")
	log.Println("Exchange declared:", EXCHANGE_NAME)

	// Declare queue
	q, err := ch.QueueDeclare(
		"",    // name
		false, // durable
		false, // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,   // arguments
	)
	FailOnError(err, "Failed to declare a queue")
	log.Println("Queue declared:", q.Name)

	// Bind queue with exchange
	err = ch.QueueBind(
		q.Name, // queue name
		"",     // routing key
		"logs", // exchange
		false,
		nil)
	FailOnError(err, "Failed to bind a queue")
	log.Println("Bound queue:", q.Name, "with exchange:", EXCHANGE_NAME)

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	FailOnError(err, "Failed to register a consumer")

	forever := make(chan bool)

	var operation Operation

	go func() {
		for d := range msgs {
			err = json.Unmarshal(d.Body, &operation)
			log.Println("Received query:", operation.Name)
			serveOperation(operation, ch, d.ReplyTo)
		}
	}()

	log.Println("Daemon running")
	<-forever
}
