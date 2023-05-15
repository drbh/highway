package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/nats-io/nats.go"
	"gopkg.in/yaml.v2"
)

type Config struct {
	NatsURL    string `yaml:"nats_url"`
	HTTPURL    string `yaml:"http_url"`
	SubChannel string `yaml:"sub_channel"`
	PubChannel string `yaml:"pub_channel"`
}

type Message struct {
	ID     string      `json:"id"`
	Msg    interface{} `json:"msg"`
	Status string      `json:"status"`
}

type ChatRequest struct {
	ChatID  int    `json:"chat_id"`
	Message string `json:"message"`
}

type ChatResponse struct {
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
		ChatID  int    `json:"chat_id"`
	} `json:"messages"`
}

var nc *nats.Conn
var js nats.JetStreamContext
var cfg Config

func main() {
	cfg = getConfig()

	var err error
	nc, err = nats.Connect(cfg.NatsURL)
	if err != nil {
		log.Fatal(err)
	}

	js, err = nc.JetStream()
	if err != nil {
		log.Fatal(err)
	}

	go processMessages()

	select {}
}

func getConfig() Config {
	data, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		log.Fatal(err)
	}

	var c Config
	err = yaml.Unmarshal(data, &c)
	if err != nil {
		log.Fatal(err)
	}

	return c
}

func processMessages() {
	sub, _ := nc.Subscribe(cfg.SubChannel, func(m *nats.Msg) {
		msg := new(Message)
		json.Unmarshal(m.Data, msg)

		req := ChatRequest{
			ChatID:  int(msg.Msg.(map[string]interface{})["chat_id"].(float64)),
			Message: msg.Msg.(map[string]interface{})["message"].(string),
		}

		resp := request(req)

		msg.Status = "Done"
		msg.Msg = resp

		msgBytes, _ := json.Marshal(msg)

		js.Publish(cfg.PubChannel, msgBytes)
	})

	sub.SetPendingLimits(-1, -1)
}

func request(request ChatRequest) ChatResponse {
	requestBytes, _ := json.Marshal(request)

	payload := strings.NewReader(string(requestBytes))

	req, _ := http.NewRequest("POST", cfg.HTTPURL, payload)

	req.Header.Add("Content-Type", "application/json")

	res, _ := http.DefaultClient.Do(req)

	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	var chatResponse ChatResponse
	json.Unmarshal(body, &chatResponse)

	return chatResponse
}
