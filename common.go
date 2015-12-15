package main

import (
	"fmt"
	"log"
)

var EXCHANGE_NAME string = "request_exchange"

type Operation struct {
	Path    string `json:"path"`
	Type    string `json:"type"`
	Check   string `json:"check"`
	Process string `json:"process"`
	Name    string `json:"name"`
}

type Response struct {
	Name     string
	Value    bool
	Hostname string
	Ip       string
}

func FailOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
		panic(fmt.Sprintf("%s: %s", msg, err))
	}
}
