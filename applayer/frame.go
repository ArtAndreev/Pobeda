package applayer

import (
	"encoding/json"
	"fmt"
	"log"

	"Pobeda/com"
	"Pobeda/datalayer"
)

type wsFrame struct {
	Type    byte            `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type wsSendFrame struct {
	Type    byte
	Payload interface{}
}

func processWSFrame(f *wsFrame) {
	switch f.Type {
	case datalayer.OP_CONNECT:
		cfg := &com.Config{}
		if err := json.Unmarshal(f.Payload, cfg); err != nil {
			log.Printf("OP_CONNECT: cannot read payload %+v: %s", f.Payload, err)
			sendAnotherError("OP_CONNECT: cannot read payload %+v: %s", f.Payload, err)
			return
		}
		sendSystemActionToLayer(datalayer.OP_CONNECT, "", cfg)
	case datalayer.OP_RING_CONNECT:
		sendSystemActionToLayer(datalayer.OP_RING_CONNECT, "", nil)
	case datalayer.OP_SEND:
		m := &message{}
		if err := json.Unmarshal(f.Payload, m); err != nil {
			log.Printf("OP_SEND: cannot read payload %+v: %s", f.Payload, err)
			sendAnotherError("OP_SEND: cannot read payload %+v: %s", f.Payload, err)
			return
		}
		sendMessageActionToLayer(m)
	case datalayer.OP_DISCONNECT:
		var port string
		if err := json.Unmarshal(f.Payload, &port); err != nil {
			log.Printf("OP_DISCONNECT: cannot cast payload %+v to string: %s", f.Payload, err)
			sendAnotherError("OP_DISCONNECT: cannot cast payload %+v to string: %s", f.Payload, err)
			return
		}
		sendSystemActionToLayer(datalayer.OP_DISCONNECT, port, nil)
	case datalayer.OP_KILL_RING:
		sendSystemActionToLayer(datalayer.OP_KILL_RING, "", nil)
	default:
		log.Printf("unknown ws frame type '%d'", f.Type)
		sendAnotherError("unknown ws frame type '%d'", f.Type)
	}
}

func sendAnotherError(format string, a ...interface{}) {
	send(&wsSendFrame{
		Type: datalayer.SystemType,
		Payload: datalayer.SystemStatus{
			Status:  datalayer.ANOTHER,
			Message: fmt.Sprintf(format, a),
		},
	})
}

func sendSystemActionToLayer(op byte, addr string, cfg *com.Config) {
	datalayer.L.AppC <- &datalayer.Action{
		AType: datalayer.SystemType,
		Data: &datalayer.SystemAction{
			Op:   op,
			Addr: addr,
			Cfg:  cfg,
		},
	}
}

func sendMessageActionToLayer(m *message) {
	datalayer.L.AppC <- &datalayer.Action{
		AType: datalayer.MessageType,
		Data: &datalayer.MessageAction{
			Addr:    m.Addr,
			Message: []byte(m.Message),
		},
	}
}
