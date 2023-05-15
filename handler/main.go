package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// Text   string `json:"text"`
type Message struct {
	ID     string      `json:"id,omitempty"`
	Msg    interface{} `json:"msg"`
	Status string      `json:"status,omitempty"`
}

var nc *nats.Conn
var js nats.JetStreamContext
var kv nats.KeyValue

const (
	URL = "yama:4222" // nats.DefaultURL
)

func main() {
	var err error

	nc, err = nats.Connect(URL)
	if err != nil {
		log.Fatal(err)
	}

	js, err = nc.JetStream()
	if err != nil {
		log.Fatal(err)
	}

	if stream, _ := js.StreamInfo("KV_discovery"); stream == nil {
		// A key-value (KV) bucket is created by specifying a bucket name.
		kv, _ = js.CreateKeyValue(&nats.KeyValueConfig{
			Bucket: "discovery",
		})
	} else {
		kv, _ = js.KeyValue("discovery")
	}

	// make sure we have a stream
	streamConfig := &nats.StreamConfig{
		Name:     "MESSAGE_STREAM",
		Subjects: []string{"cli.*"}, // This will catch cli.demo and cli.done
	}

	// Creating the stream
	_, err = js.AddStream(streamConfig)
	if err != nil {
		log.Fatal(err)
	}

	// create new fiber app
	app := fiber.New()

	app.Use("/ws", func(c *fiber.Ctx) error {
		// IsWebSocketUpgrade returns true if the client
		// requested upgrade to the WebSocket protocol.
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	// here we listen for incoming HTTP POST requests and publish them to the NATS stream
	// then we immediately return a 200 OK to the client with a UUID
	//
	// This essentially queues incoming requests and lets the worker process them in the background
	app.Post("/trigger", func(c *fiber.Ctx) error {
		msg := new(Message)
		if err := c.BodyParser(msg); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Cannot parse JSON",
			})
		}

		msg.ID = uuid.New().String()
		msg.Status = "Processing"

		fmt.Println("Publishing message: ", msg)

		msgBytes, err := json.Marshal(msg)
		if err != nil {
			log.Printf("Error marshaling message: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error marshaling message",
			})
		}

		_, err = js.Publish("cli.demo", msgBytes)
		if err != nil {
			log.Printf("Error publishing message: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Cannot publish message",
			})
		}

		return c.JSON(msg)
	})

	// here the user can exchange the UUID for the status of the message
	app.Get("/done/:uuid", func(c *fiber.Ctx) error {
		uuid := c.Params("uuid")
		if uuid == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Missing uuid",
			})
		}

		entry, err := kv.Get(uuid)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Cannot get KV",
			})
		}

		// delete the key
		err = kv.Delete(uuid)
		if err != nil {
			log.Printf("Error deleting key: %v", err)
		}

		// turn entry.Value() into a Message struct
		msg := new(Message)
		err = json.Unmarshal(entry.Value(), msg)
		if err != nil {
			log.Printf("Error unmarshaling message: %v", err)

		}

		return c.JSON(msg)
	})

	// this is a websocket endpoint that will listen for messages from the NATS stream
	// and immediately send them to the client
	app.Get("/ws/:id", websocket.New(func(c *websocket.Conn) {
		// c.Locals is added to the *websocket.Conn
		log.Println(c.Locals("allowed"))  // true
		log.Println(c.Params("id"))       // 123
		log.Println(c.Query("v"))         // 1.0
		log.Println(c.Cookies("session")) // ""

		// websocket.Conn bindings https://pkg.go.dev/github.com/fasthttp/websocket?tab=doc#pkg-index
		var (
			mt  int
			msg []byte
			err error
		)
		for {
			if mt, msg, err = c.ReadMessage(); err != nil {
				log.Println("read:", err)
				break
			}
			log.Printf("recv: %s", msg)

			// unmarsal the message into Message struct
			fullMessage := new(Message)
			err = json.Unmarshal(msg, fullMessage)
			if err != nil {
				log.Printf("Error unmarshaling message: %v", err)
				break
			}

			// add an id if it doesn't exist
			if fullMessage.ID == "" {
				fullMessage.ID = uuid.New().String()
			}

			fullMessage.Status = "Processing"

			// emit the message to NATS after building a Message struct
			msgBytes, err := json.Marshal(fullMessage)
			if err != nil {
				log.Printf("Error marshaling message: %v", err)
				break
			}

			_, err = js.Publish("cli.demo", msgBytes)
			if err != nil {
				log.Printf("Error publishing message: %v", err)
				break
			}

			// emit the id once we post the message to NATS
			err = c.WriteMessage(mt,
				[]byte(`{"id":"`+fullMessage.ID+`","status":"started"}`),
			)
			if err != nil {
				log.Println("write:", err)
				break
			}

			fmt.Println("mt: ", mt)

			var entry nats.KeyValueEntry

			// wait for the message to come back from NATS
			// and send it to the client. do this by polling the KV store
			for {
				entry, err = kv.Get(fullMessage.ID)
				if err != nil {
					log.Printf("Error getting message: %v", err)
					// wait 2 seconds
					time.Sleep(2 * time.Second)
					continue
				}

				// remove the message from the KV store
				err = kv.Delete(fullMessage.ID)
				if err != nil {
					log.Printf("Error deleting message: %v", err)
				}

				break
			}

			fmt.Println("Publishing message: ", entry)

			if err = c.WriteMessage(mt,
				entry.Value(),
			); err != nil {
				log.Println("write:", err)
				break
			}
		}

	}))

	go processMessages()

	app.Listen(":3000")
}

func processMessages() {
	nc.Subscribe("cli.done", func(m *nats.Msg) {
		fmt.Println("Received message: ", string(m.Data))
		msg := new(Message)
		json.Unmarshal(m.Data, msg)
		kv.Put(msg.ID, m.Data)
	})
}
