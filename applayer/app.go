package applayer

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/satori/go.uuid"
)

var (
	// for handling multiple browser sessions
	clients = make(map[string]Client, 2)
)

func Connect(w http.ResponseWriter, r *http.Request) {
	u := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := u.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Cannot upgrade connection:", err)
		return
	}

	handle(conn)
}

func handle(conn *websocket.Conn) {
	c := Client{
		uuid:  uuid.NewV4().String(),
		conn:  conn,
		sendC: make(chan []byte, queueLen),
	}
	go c.Listen()
	go c.Send()
	clients[c.uuid] = c
}

func send(m *wsSendFrame) {
	j, err := json.Marshal(m)
	if err != nil {
		log.Printf("cannot json marshal %T", m)
		return
	}
	log.Printf("clients: %+v", clients)
	for _, v := range clients {
		v.sendC <- j
	}
}

func disconnect(uuid string) {
	delete(clients, uuid)
}
