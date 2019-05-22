package applayer

import (
	"encoding/json"
	"log"

	"Pobeda/com"
	"Pobeda/datalayer"
)

type wsFrame struct {
	Type    byte            `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type wsSendFrame struct {
	Type    byte        `json:"type"`
	Payload interface{} `json:"payload"`
}

func processWSFrame(f *wsFrame) {
	switch f.Type {
	case datalayer.OP_CONNECT:
		cfg := &com.Config{}
		if err := json.Unmarshal(f.Payload, cfg); err != nil {
			log.Printf("OP_CONNECT: cannot read payload %+v: %s", f.Payload, err)
			datalayer.SendActionStatusToApp(datalayer.ERROR, "", "", datalayer.ErrProtocolBug)
			return
		}
		datalayer.GetActionStatusFromApp(datalayer.OP_CONNECT, "", cfg, "")
	case datalayer.OP_RING_CONNECT:
		datalayer.GetActionStatusFromApp(datalayer.OP_RING_CONNECT, "", nil, "")
	case datalayer.OP_SEND:
		m := &message{}
		if err := json.Unmarshal(f.Payload, m); err != nil {
			log.Printf("OP_SEND: cannot read payload %+v: %s", f.Payload, err)
			datalayer.SendActionStatusToApp(datalayer.ERROR, "", "", datalayer.ErrProtocolBug)
			return
		}
		datalayer.GetActionStatusFromApp(datalayer.OP_SEND, m.Addr, nil, m.Message)
	case datalayer.OP_DISCONNECT:
		var port string
		if err := json.Unmarshal(f.Payload, &port); err != nil {
			log.Printf("OP_DISCONNECT: cannot cast payload %+v to string: %s", f.Payload, err)
			datalayer.SendActionStatusToApp(datalayer.ERROR, "", "", datalayer.ErrProtocolBug)
			return
		}
		datalayer.GetActionStatusFromApp(datalayer.OP_DISCONNECT, port, nil, "")
	case datalayer.OP_KILL_RING:
		datalayer.GetActionStatusFromApp(datalayer.OP_KILL_RING, "", nil, "")
	default:
		log.Printf("unknown ws frame type '%d'", f.Type)
		datalayer.SendActionStatusToApp(datalayer.ERROR, "", "", datalayer.ErrProtocolBug)
	}
}
