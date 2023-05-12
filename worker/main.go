package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

type Message struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Status string `json:"status"`
}

var nc *nats.Conn
var js nats.JetStreamContext

func main() {
	var err error
	nc, err = nats.Connect("yama:4222")
	// nc, err = nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatal(err)
	}

	js, err = nc.JetStream()
	if err != nil {
		log.Fatal(err)
	}

	go processMessages()

	// Keep the service running
	select {}
}

func processMessages() {
	sub, _ := nc.Subscribe("cli.demo", func(m *nats.Msg) {
		msg := new(Message)
		json.Unmarshal(m.Data, msg)

		randomIntBelow10 := time.Now().Unix() % 10

		fmt.Printf("Processing message: %s\n", msg.Text)
		fmt.Printf("Sleeping for %d seconds\n", randomIntBelow10)

		// sleep that many seconds
		time.Sleep(time.Duration(randomIntBelow10) * time.Second)

		fmt.Printf("Done processing message: %s\n", msg.Text)

		msg.Status = "Done"
		msgBytes, _ := json.Marshal(msg)

		js.Publish("cli.done", msgBytes)
	})

	// Make sure messages are processed in order
	sub.SetPendingLimits(-1, -1)
}
