package datalayer

import (
	"Pobeda/com"
)

const (
	SystemType = iota
	MessageType
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
	ANOTHER                   // errors, protocol bugs
	CONNECT_RING              // connected ring
)

// system operations to perform from app layer to data layer
const (
	OP_CONNECT      = iota // physical
	OP_DISCONNECT          // physical
	OP_RING_CONNECT        // logical
	OP_KILL_RING           // logical
	OP_SEND                // matches to MessageType
)

type MessageAction struct {
	Addr    string
	Message []byte
}

type SystemAction struct {
	Op byte `json:"op"` // required

	Addr string      `json:"addr"` // disconnect
	Cfg  *com.Config `json:"cfg"`  // connect
}

type SystemStatus struct {
	Status  byte
	Message string
}
