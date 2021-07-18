package server

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"
)

var SSEBroker Broker

const cloudflareTimeout time.Duration = 100 * time.Second

func init() {
	// Tick a few seconds before cloudflare times out the connection
	timer := time.Tick(cloudflareTimeout - 5*time.Second)
	SSEBroker = Broker{
		make(chan ServerEvent),
		make(map[string][]client),
		make(chan client),
		make(chan client),
		make(chan os.Signal),
		timer,
	}
	go SSEBroker.Start()
	signal.Notify(SSEBroker.sigInt, os.Interrupt)
}

type ServerEvent struct {
	Destination string
	Data        []byte
}

type client struct {
	Source string
	Msg    chan []byte
	SigInt chan struct{}
	Ping   chan struct{}
}

type Broker struct {
	Event     chan ServerEvent
	clients   map[string][]client
	subCh     chan client
	unsubCh   chan client
	sigInt    chan os.Signal
	pingTimer <-chan time.Time
}

func (b Broker) Start() {
	for {
		select {
		case <-b.pingTimer:
			b.ping()
		case event := <-b.Event:
			b.broadcast(event)
		case newClient := <-b.subCh:
			b.add(newClient)
		case removeClient := <-b.unsubCh:
			b.remove(removeClient)
		case <-b.sigInt:
			b.shutdown()
			return
		}
	}
}

// Send an empty message to all connected clients to refresh connection timer
func (b Broker) ping() {
	for _, e := range b.clients {
		for _, f := range e {
			f.Ping <- struct{}{}
		}
	}
}

// Send event data to all relevant clients
func (b Broker) broadcast(event ServerEvent) {
	for _, e := range b.clients[event.Destination] {
		e.Msg <- event.Data
	}
}

// Add channel to collection of clients
func (b Broker) Add(newClient client) {
	b.subCh <- newClient
}

func (b *Broker) add(newClient client) {
	clients, exist := b.clients[newClient.Source]
	if !exist {
		b.clients[newClient.Source] = []client{newClient}
	} else {
		b.clients[newClient.Source] = append(clients, newClient)
	}
}

// Close the channel and remove it from the collection of clients
func (b Broker) Remove(remove client) {
	close(remove.Msg)
	close(remove.SigInt)
	b.unsubCh <- remove
}

func (b *Broker) remove(remove client) {
	clients := b.clients[remove.Source]
	for i, e := range clients {
		if remove.Msg == e.Msg {
			// If removing last connection from an endpoint, remove from map
			if len(clients) == 1 {
				delete(b.clients, remove.Source)
			} else {
				clients[i] = clients[len(clients)-1]
				b.clients[remove.Source] = clients[:len(clients)-1]
			}
			return
		}
	}
}

// Close all open connections
func (b *Broker) shutdown() {
	for _, e := range b.clients {
		for _, f := range e {
			f.SigInt <- struct{}{}
		}
	}
}

// Subscribe client to relevant sse
func sse(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		httpError(w, r, fmt.Errorf("flushing unavailable"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")

	// Differentiate which type of events to listen to by source url
	u, err := url.Parse(r.Referer())
	if err != nil {
		httpError(w, r, err)
		return
	}

	listener := client{
		Source: u.Path,
		Msg:    make(chan []byte),
		SigInt: make(chan struct{}),
		Ping:   make(chan struct{}),
	}

	SSEBroker.Add(listener)

	for {
		select {
		case <-listener.Ping:
			// Send a comment line to stop cloudflare killing the connection
			fmt.Fprint(w, ":")
			flusher.Flush()
		case msg := <-listener.Msg:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-r.Context().Done():
			SSEBroker.Remove(listener)
			return
		case <-listener.SigInt:
			return
		}
	}
}
