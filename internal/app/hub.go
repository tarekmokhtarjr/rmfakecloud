package app

import (
	"net/http"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/ddvk/rmfakecloud/internal/messages"
	"github.com/gorilla/websocket"
)

// Hub ws notificaiton hub
type Hub struct {
	clients      map[*wsClient]bool
	userClients  map[string]map[*wsClient]bool
	additions    chan *wsClient
	removals     chan *wsClient
	notification chan *messages.WsMessage
}

// Send sends a message to all connected clients
func (h *Hub) Send(uid string, msg messages.WsMessage) {
	log.Info("Sending notification, uid:", uid)
	if clients, ok := h.userClients[uid]; ok {
		for c := range clients {
			c.ntf <- msg
		}
	}
}

// ClientCount number of connected clients
func (h *Hub) ClientCount() int {
	return len(h.clients)
}

// NewHub construct a hub
func NewHub() *Hub {
	h := Hub{
		clients:     make(map[*wsClient]bool),
		userClients: make(map[string]map[*wsClient]bool),
		additions:   make(chan *wsClient),
		removals:    make(chan *wsClient),
	}
	go h.start()
	return &h
}

func (h *Hub) removeClient(c *wsClient) {
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.ntf)
	}
	if userclients, ok := h.userClients[c.uid]; ok {
		delete(userclients, c)
		if len(userclients) == 0 {
			delete(h.userClients, c.uid)
		}
	}
}
func (h *Hub) start() {
	for {
		select {
		case c := <-h.additions:
			log.Info("hub: adding a client")
			h.clients[c] = true
			h.userClients[c.uid][c] = true
		case c := <-h.removals:
			h.removeClient(c)
		}
	}
}

type wsClient struct {
	uid string
	ntf chan messages.WsMessage
	hub *Hub
}

func (c *wsClient) Read(ws *websocket.Conn) {
	// defer ws.Close()
	for {
		_, p, err := ws.ReadMessage()
		if err != nil {
			log.Warn("Hub Read ", err)
			//c.hub.removals <- c
			return
		}
		log.Println("Message: ", string(p))
	}
}
func (c *wsClient) Write(ws *websocket.Conn) {
	// defer ws.Close()
	for {
		select {
		case m, ok := <-c.ntf:
			if !ok {
				log.Debugln("done")
				return
			}
			log.Debugln("sending notification")
			ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			err := ws.WriteJSON(m)
			if err != nil {
				log.Warn("Hub write ", err)
				//c.hub.removals <- c
				return
			}
		}
	}
}

// ConnectWs upgrade the connection to websocket
func (h *Hub) ConnectWs(uid string, w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	client := &wsClient{
		uid: uid,
		hub: h,
		ntf: make(chan messages.WsMessage),
	}
	h.additions <- client
	go client.Read(c)
	client.Write(c)
	c.Close()
	log.Info("normal end")
	h.removals <- client
}

func newWs(doc *messages.RawDocument, typ string) messages.WsMessage {
	tt := time.Now().UTC().Format(time.RFC3339Nano)
	msg := messages.WsMessage{
		Message: messages.NotificationMessage{
			MessageId:  "1234",
			MessageId2: "1234",
			Attributes: messages.Attributes{
				Auth0UserID:      "auth0|12341234123412",
				Event:            typ,
				Id:               doc.Id,
				Type:             doc.Type,
				Version:          strconv.Itoa(doc.Version),
				VissibleName:     doc.VissibleName,
				SourceDeviceDesc: "some-client",
				SourceDeviceId:   "12345",
				Parent:           doc.Parent,
			},
			PublishTime:  tt,
			PublishTime2: tt,
		},
		Subscription: "dummy-subscription",
	}

	return msg
}
