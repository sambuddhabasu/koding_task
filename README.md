# Koding Task 1
---
On each of the servers, to run the daemon, do `make daemon`.

All of the operation queries need to be in the `operation.txt` file.
To query all the machines, we will do `make query`. This will prompt us for the number of servers in the network. Once we enter that, the program uses RabbitMQ to message all of the servers and waits for their reply.

Appropriate log messages are printed out to `stdout`.

**Note**: To install the amqp package, do `go get github.com/streadway/amqp`.
