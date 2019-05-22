package com

import (
	"log"
)

var (
	L layer
)

const (
	queueLen = 32
)

type layer struct {
	SendC chan *SendInfo
	GotC  chan *SendInfo
}

func newLayer(len int) layer {
	return layer{
		SendC: make(chan *SendInfo, len),
		GotC:  make(chan *SendInfo, len),
	}
}

func (l *layer) listenToDataLinkLayer() {
	for m := range l.SendC {
		if err := write(m.Name, m.Data); err != nil {
			// blah-blah-blah
			if err == ErrConnNotFound { // or dead
				// send disconnect
			}
			log.Printf("com: cannot write to port: %s", err) // dead port, etc.
			continue
		}
		log.Printf("send %x to %s", m.Data, m.Name)
	}
}

type SendInfo struct {
	Name string
	Data []byte
}

func Init() {
	L = newLayer(queueLen)
	go L.listenToDataLinkLayer()
}

func Close() {
	for c := range conns {
		if err := ClosePort(c); err != nil {
			log.Printf("close port %s err: %s", c, err)
		}
	}

	close(L.SendC)
	close(L.GotC)
}
