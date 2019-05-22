package applayer

import (
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"
)

type Client struct {
	uuid  string
	conn  *websocket.Conn
	sendC chan []byte
}

func (c *Client) Listen() {
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err) {
				log.Printf("listen: client %s was disconnected", c.uuid)
			} else {
				log.Printf("listen: client %s unknown err: %s", c.uuid, err)
			}
			disconnect(c.uuid)
			close(c.sendC)
			return
		}

		m := &wsFrame{}
		if err = json.Unmarshal(raw, m); err != nil {
			log.Println("cannot unmarshal", err)
			continue
		}

		processWSFrame(m)
	}
}

func (c *Client) Send() {
	for m := range c.sendC {
		err := c.conn.WriteMessage(websocket.TextMessage, m)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err) {
				log.Printf("send: client %s was disconnected", c.uuid)
			} else {
				log.Printf("send: client %s unknown err: %s", c.uuid, err)
			}
			disconnect(c.uuid)
			return
		}
	}
}
