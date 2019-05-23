package datalayer

import (
	"Pobeda/com"
)

type Action struct {
	AType byte
	Data  interface{}
}

// system statuses to app layer from bottom layers
const (
	NO_ACK             = iota // send message failure
	DISCONNECT                // disconnect
	DISRUPTION                // ring disruption
	CONNECT                   // successfully connected
	CONNECT_REQUEST           //
	DISCONNECT_REQUEST        //
	ACK                       // send message ok
	ERROR                     // errors, protocol bugs
	CONNECT_RING              // connected ring

	MESSAGE // message to frontend
)

// for ERROR
const (
	ErrProtocolBug = "ErrProtocolBug"
	ErrPhysConnect = "ErrPhysConnect"
	ErrRingConnect = "ErrRingConnect"
)

// system operations to perform from app layer to data layer
const (
	OP_CONNECT      = iota // physical
	OP_DISCONNECT          // physical
	OP_RING_CONNECT        // logical
	OP_KILL_RING           // logical
	OP_SEND                // message
)

type SystemAction struct { // from frontend
	Addr    string      `json:"addr"` // disconnect
	Cfg     *com.Config `json:"cfg"`  // connect
	Message string      `json:"message"`
}

type ActionPayload struct {
	Addr    string `json:"addr,omitempty"`
	Message string `json:"message,omitempty"`
	To      string `json:"to,omitempty"`
}
