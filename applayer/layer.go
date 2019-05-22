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

type layer struct{}

func (l *layer) listenToDataLinkLayer() {
	var f *wsSendFrame
	for a := range datalayer.L.GetAppC {
		status, ok := a.Data.(*datalayer.ActionPayload)
		if !ok {
			log.Printf("cannot cast %T to *datalayer.ActionPayload", a.Data)
			continue
		}
		f = &wsSendFrame{
			Type:    a.AType,
			Payload: status,
		}

		send(f)
	}
}

func Init() {
	L = layer{}
	go L.listenToDataLinkLayer()
}
