# highway

This is a simple set of services that use NATs to communicate between each other.

The idea is to have a simple way to trigger a long running process and then get the results back.

highway is a work in progress and is not ready for production.

## General Goal

highway aims to be part of a larger overlay network that abstracts away the complexity of physical and virtual networks.

How's it work.

1. Tailscale/Wireguard on all physical nodes
2. NAT with Jetstream as a central message broker
3. highway as a WS and HTTP handler as broker to the NATS network
4. highway as a worker that can be triggered by messages

## What does it do?

You can connect to the websocket and send a message to the server. The server will then send a message back with the id of the job and the status. Once the job is done, the server will send a message with the id of the job, the status and the results.

Otherwise you can request the server to start a job by sending a POST request to the `/trigger` endpoint. The server will then send a message back with the id of the job and the status. Then you can poll the `/done/:id` endpoint to get the results of the job.

In both cases the message is deleted from the queue once the value has been sent back to the client.

## How to run it?

```bash
# terminal 1
cd worker
go run main.go
```

```bash
# terminal 2
cd handler
go run main.go
```

## Interacting with the `handler`

#### Trigger a job

```bash
curl --request POST \
  --url http://localhost:3000/trigger \
  --header 'Content-Type: application/json' \
  --data '{
	"text": "hello david"
}'

# {"id":"c9ae7ca7-15ee-4fbe-a6b2-9782fe69e07a","text":"hello david","status":"Processing"}
```

#### Check the status of the job

```bash
curl --request GET \
  --url http://localhost:3000/done/dab6021c-f6ec-4ae1-b79c-231eedc0cdf1

# {"id":"dab6021c-f6ec-4ae1-b79c-231eedc0cdf1","text":"hello david","status":"Done"}
```

#### Via Websocket

```bash
websocat ws://127.0.0.1:3000/ws/0 -v
# [INFO  websocat::lints] Auto-inserting the line mode
# [INFO  websocat::stdio_threaded_peer] get_stdio_peer (threaded)
# [INFO  websocat::ws_client_peer] get_ws_client_peer
# [INFO  websocat::ws_client_peer] Connected to ws
# {"text":"what color is the sky?"}
# {"id":"f17d8221-d9e3-40ec-ad87-ca55b244e589","status":"started"}
# {"id":"f17d8221-d9e3-40ec-ad87-ca55b244e589","text":"what color is the sky?","status":"Done"}
```
