package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"bufio"
	"fmt"
	"github.com/dcarbone/turnpike"
	"strconv"
	"strings"
)

var (
	realm  string
	port   int
	debug  bool
	client bool
)

func init() {
	flag.StringVar(&realm, "realm", "realm1", "realm name")
	flag.IntVar(&port, "port", 8000, "port to run on")
	flag.BoolVar(&debug, "debug", false, "enable debug logging")
	flag.BoolVar(&client, "client", false, "run client")
}

func main() {
	flag.Parse()
	if debug {
		turnpike.Debug()
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt)

	if client {
		log.Printf("Starting Local Turnpike Client")
		log.Printf("  Realm: %s", realm)
		log.Printf("  Port: %d", port)
		log.Printf("  Debug: %s", strconv.FormatBool(debug))

		c, err := turnpike.NewWebSocketClient(turnpike.SerializationFormatJSON, fmt.Sprintf("ws://localhost:%d/", port), nil, nil)
		if nil != err {
			log.Printf("Unable to create client: %s", err)
			os.Exit(1)
		}

		go func() {
			<-shutdown
			c.LeaveRealm()
			c.Close()
			log.Printf("Closing client...")
			time.Sleep(time.Second)
			os.Exit(1)
		}()

		log.Printf("Attempting to join realm \"%s\"...", realm)

		_, err = c.JoinRealm(realm, nil)
		if nil != err {
			log.Printf("Error joining realm \"%s\": %s", realm, err)
			os.Exit(1)
		}

		go c.Receive()

		reader := bufio.NewReader(os.Stdin)

		for {
			fmt.Println("Enter command:")
			text, _ := reader.ReadString('\n')
			if "" == text {
				log.Printf("Saw empty input")
				continue
			}

			text = strings.TrimSpace(text)

			fmt.Printf("Saw: %s\n", text)

			split := strings.Split(text, " ")

			fmt.Printf("Saw: %v\n", split)

			splitLen := len(split)

			fmt.Printf("Split length: %d\n", splitLen)

			if 0 == splitLen {
				fmt.Println("Invalid command format, expected \"action input...\"")
				continue
			}

			switch split[0] {
			case "subscribe":
				if 2 == splitLen {
					err = c.Subscribe(split[1], nil, func(e *turnpike.Event) {
						log.Printf("Topic \"%s\" saw message: %v\n", split[1], e)
					})
					if nil != err {
						log.Printf("Unable to subscribe to topic \"%s\": %s", split[1], err)
					} else {
						log.Printf("Subcribed to topic \"%s\"", split[1])
					}
				} else {
					fmt.Println("\"subscribe\" expects \"subscribe topicname\"")
				}
			case "publish":
				if 3 <= splitLen {
					err = c.Publish(split[1], nil, []interface{}{strings.Join(split[2:], " ")}, nil)
					if nil != err {
						log.Printf("Unable to publish to topic \"%s\": %s", split[1], err)
					} else {
						log.Printf("Published to topic \"%s\"", split[1])
					}
				} else {
					fmt.Println("\"publish\" expects \"publish topicname payload\"")
				}
			}
		}

	} else {

		log.Printf("Starting Local Turnpike WebSocketSever...")
		log.Printf("  Realm: %s", realm)
		log.Printf("  Port: %d", port)
		log.Printf("  Debug: %s", strconv.FormatBool(debug))

		s := turnpike.NewBasicWebSocketServer(realm)

		go func() {
			<-shutdown
			s.Close()
			log.Println("shutting down server...")
			time.Sleep(time.Second)
			os.Exit(1)
		}()

		server := &http.Server{
			Handler: s,
			Addr:    ":8000",
		}

		log.Printf("turnpike server starting on port %d...", port)
		log.Fatal(server.ListenAndServe())
	}
}
