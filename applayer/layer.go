package applayer

import (
	"log"

	"Pobeda/datalayer"
)

const (
	queueLen = 32
)

var (
	L layer
)

type layer struct {
	GetC chan *datalayer.Action
}

func newLayer(len int) layer {
	return layer{
		GetC: make(chan *datalayer.Action, len),
	}
}

func (l *layer) listenToDataLinkLayer() {
	var f *wsSendFrame
	for a := range l.GetC {
		switch a.AType {
		case datalayer.MessageType:
			msg, ok := a.Data.(*datalayer.MessageAction)
			if !ok {
				log.Printf("cannot cast %T to *datalayer.MessageAction", a.Data)
				continue
			}
			f = &wsSendFrame{
				Type:    datalayer.MessageType,
				Payload: msg,
			}
		case datalayer.SystemType:
			status, ok := a.Data.(*datalayer.SystemStatus)
			if !ok {
				log.Printf("cannot cast %T to *datalayer.SystemStatus", a.Data)
				continue
			}
			f = &wsSendFrame{
				Type:    datalayer.MessageType,
				Payload: status,
			}
		}

		send(f)
	}
}

func Init() {
	L = newLayer(queueLen)
	go L.listenToDataLinkLayer()
}

func Close() {
	close(L.GetC)
}
