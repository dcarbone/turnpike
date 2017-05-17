package main

import (
	"log"

	"github.com/dcarbone/turnpike"
)

func main() {
	c, err := turnpike.NewWebSocketClient(turnpike.SerializationFormatJSON, "ws://localhost:8000/", nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("connected to router")
	_, err = c.JoinRealm("turnpike.examples", nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("joined realm")
	c.ReceiveDone = make(chan bool)

	onJoin := func(e *turnpike.Event) {
		log.Println("session joined:", e.Arguments[0])
	}
	if err := c.Subscribe("wamp.session.on_join", nil, onJoin); err != nil {
		log.Fatalln("Error subscribing to channel:", err)
	}

	onLeave := func(e *turnpike.Event) {
		log.Println("session left:", e.Arguments[0])
	}
	if err := c.Subscribe("wamp.session.on_leave", nil, onLeave); err != nil {
		log.Fatalln("Error subscribing to channel:", err)
	}

	log.Println("listening for meta events")
	<-c.ReceiveDone
	log.Println("disconnected")
}
