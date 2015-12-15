package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/streadway/amqp"
)

var EXCHANGE_NAME string = "request_exchange"
var QUERIES int = 0
var QUERY_DONE int = 0

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
		panic(fmt.Sprintf("%s: %s", msg, err))
	}
}

func readLines(path string) (string, error) {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return string(contents), err
	}
	return string(contents), nil
}

type Operation struct {
	Path    string `json:"path"`
	Type    string `json:"type"`
	Check   string `json:"check"`
	Process string `json:"process"`
	Name    string `json:"name"`
}

type OperationCollection struct {
	OperationName map[string]*Operation
}

type Response struct {
	Name     string
	Value    bool
	Hostname string
	Ip       string
}

func main() {
	var servers int
	fmt.Printf("Enter number of servers: ")
	fmt.Scanf("%d\n", &servers)
	log.Println("Number of servers:", servers)

	// Connect to RabbitMQ
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()
	log.Println("Connected to RabbitMQ")

	// Create a channel
	channel, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer channel.Close()
	log.Println("Opened a channel")

	// Setup exchange
	err = channel.ExchangeDeclare(
		EXCHANGE_NAME, // name
		"fanout",      // type
		true,          // durable
		false,         // auto-deleted
		false,         // internal
		false,         // no-wait
		nil,           // arguments
	)
	failOnError(err, "Failed to declare an exchange")
	log.Println("Exchange declared:", EXCHANGE_NAME)

	// Dec;are the response queue
	resp_q, err := channel.QueueDeclare(
		"",    // name
		false, // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	failOnError(err, "Failed to declare a queue")
	log.Println("Queue declared:", resp_q.Name)

	msgs, err := channel.Consume(
		resp_q.Name, // queue
		"",          // consumer
		true,        // auto-ack
		false,       // exclusive
		false,       // no-local
		false,       // no-wait
		nil,         // args
	)
	failOnError(err, "Failed to register a consumer")

	// Read from operations file
	lines, err := readLines("operation.txt")
	failOnError(err, "Failed to read the operations file")

	// Parse the operations
	var operations OperationCollection
	bytes := []byte(lines)
	err = json.Unmarshal(bytes, &operations.OperationName)
	failOnError(err, "Failed to parse the operations")
	log.Println("Read and parsed queries")

	var result map[string][]Response = make(map[string][]Response)

	// Publish the messages one by one
	for k := range operations.OperationName {
		QUERIES++
		operations.OperationName[k].Name = k
		result[k] = []Response{}
		body, _ := json.Marshal(operations.OperationName[k])
		err = channel.Publish(
			"logs", // exchange
			"",     // routing key
			false,  // mandatory
			false,  // immediate
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte(body),
				ReplyTo:     resp_q.Name,
			})
		failOnError(err, fmt.Sprintf("Failed to publish message: %s", body))
		log.Println("Querying:", k)
	}

	forever := make(chan bool)

	var resp Response
	var passNum, failNum int

	go func() {
		for d := range msgs {
			err = json.Unmarshal(d.Body, &resp)
			result[resp.Name] = append(result[resp.Name], resp)
			if len(result[resp.Name]) == servers {
				log.Println("Query complete:", resp.Name)
				fmt.Println("----------", resp.Name, "----------")
				passNum = 0
				failNum = 0
				for e := range result[resp.Name] {
					if result[resp.Name][e].Value == true {
						passNum++
					} else {
						failNum++
					}
				}
				fmt.Println("Servers passed:", passNum)
				fmt.Println("Servers failed:", failNum)
				if failNum != 0 {
					fmt.Println("List of servers failed:")
					for e := range result[resp.Name] {
						if result[resp.Name][e].Value == false {
							fmt.Printf("%s@%s\n", result[resp.Name][e].Hostname, result[resp.Name][e].Ip)
						}
					}
				}
				fmt.Println("------------------------------")
				QUERY_DONE++
			}
			if QUERY_DONE == QUERIES {
				log.Println("Finished all queries")
				os.Exit(0)
			}
		}
	}()

	<-forever
}
