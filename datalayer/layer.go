package datalayer

import (
	"fmt"
	"log"

	"Pobeda/com"
)

const (
	queueLen = 32
)

var (
	L layer

	lastFrame []byte
)

type layer struct {
	SendAppC chan *Action
	GetAppC  chan *Action
	QueueLen int

	myAddr    byte
	conns     map[string]byte
	lastDead  string // for messages from another peer to another peer (1 -> me ...dc... 3)
	delivered chan struct{}
}

func newLayer(len int) layer {
	return layer{
		SendAppC: make(chan *Action, len),
		GetAppC:  make(chan *Action, len),
		QueueLen: len,

		myAddr:    0,
		conns:     make(map[string]byte, 2),
		delivered: make(chan struct{}),
	}
}

func (l *layer) listenToAppLayer() {
	for a := range l.SendAppC {
		switch a.AType {
		case SystemType:
			sa, ok := a.Data.(*SystemAction)
			if !ok {
				log.Printf("cannot cast to *SystemAction '%T'", sa)
				continue
			}

			switch sa.Op {
			case OP_CONNECT:
				if sa.Cfg == nil {
					log.Printf("cannot connect: no cfg available")
					sendAnotherErrorToApp("cannot connect: no cfg available")
					continue
				}
				if err := com.Connect(sa.Cfg); err != nil {
					log.Printf("cannot connect to %s: %s", sa.Cfg.Name, err)
					sendAnotherErrorToApp("cannot connect to %s: %s", sa.Cfg.Name, err)
					continue
				}
				sendSystemStatusToApp(CONNECT, sa.Addr)
			case OP_DISCONNECT:
				if err := com.ClosePort(sa.Addr); err != nil {
					log.Printf("cannot disconnect from %s: %s", sa.Addr, err)
					sendAnotherErrorToApp("cannot disconnect from %s: %s", sa.Addr, err)
					continue
				}
				log.Printf("successful disconnect from %s", sa.Addr)
				sendSystemStatusToApp(DISCONNECT, sa.Addr)
				// disconnect gracefully killing the ring
				killRing()
			case OP_RING_CONNECT:
				port := L.getRandomPortName()
				if port == "" {
					sendAnotherErrorToApp("cannot ring connect: no port available")
					continue
				}
				if L.myAddr == 0 { // better think about it
					L.myAddr = 1
					f, err := newFrame(0, L.myAddr, linkFrame, nil)
					if err != nil {
						log.Printf("cannot ring connect: %s", err)
						sendAnotherErrorToApp("cannot ring connect: %s", err)
						continue
					}
					sendToPort(port, f.Marshal())
				} else {
					log.Printf("cannot ring connect: already connected")
					sendAnotherErrorToApp("cannot ring connect: already connected")
				}
			case OP_KILL_RING:
				killRing()
			default:
				log.Printf("system action: unknown op type %d", a.AType)
				sendAnotherErrorToApp("system action: unknown op type %d", a.AType)
			}

		case MessageType:
			ma, ok := a.Data.(*MessageAction)
			if !ok {
				log.Printf("cannot cast to *MessageAction '%T'", ma)
				continue
			}
			var addr byte
			if ma.Addr != "" { // broadcast
				var ok bool
				addr, ok = L.findAddrByPortName(ma.Addr)
				if !ok {
					log.Printf("cannot send message to %s", ma.Addr)
					sendSystemStatusToApp(NO_ACK, "disconnected")
					sendSystemStatusToApp(DISCONNECT, ma.Addr)
					L.kickDeadConn(ma.Addr)
					killRing()
					continue
				}
			} else {
				addr = broadcast
			}
			f, err := newFrame(L.myAddr, addr, iFrame, ma.Message)
			if err != nil {
				log.Printf("cannot put message to frame: %s", err)
				sendAnotherErrorToApp("cannot put message to frame: %s", err)
				continue
			}
			sendToPort(ma.Addr, f.Marshal())

			// let's wait for delivery
			<-l.delivered
			lastFrame = nil
			// send next message

		default:
			log.Printf("unknown action type %d", a.AType)
			sendAnotherErrorToApp("unknown action type %d", a.AType)
		}
	}
}

func killRing() {
	port := L.getRandomPortName()
	if port == "" {
		sendAnotherErrorToApp("cannot ring disconnect: no port available")
		return
	}
	if L.myAddr != 0 {
		f, err := newFrame(0, L.myAddr, linkFrame, nil)
		if err != nil {
			sendAnotherErrorToApp("cannot ring disconnect: %s", err)
			return
		}
		L.myAddr = 0
		sendToPort(port, f.Marshal())
	} else {
		sendAnotherErrorToApp("cannot ring disconnect: already disconnected")
	}

	sendSystemStatusToApp(DISRUPTION, "OK")
}

func (l *layer) kickDeadConn(name string) {
	delete(l.conns, name)
	l.lastDead = name
}

func (l *layer) listenToPhysLayer() {
	var buf []byte
	started := false
	for got := range com.L.GotC {
		log.Printf("got from phys layer: %+v", got)
		if !started {
			if isValidFrame(got.Data) {
				started = true
			} else {
				// broken, skip
				continue
			}
		}
		buf = append(buf, got.Data...)
		if end := findEndOfFrame(got.Data); end != -1 {
			// gotcha frame
			buf = append(buf, got.Data[:end+1]...)
			res := make([]byte, len(buf))
			copy(res, buf)

			buf = []byte{}
			got.Data = got.Data[end+1:]
			if isValidFrame(got.Data) {
				buf = append(buf, got.Data...)
			}

			log.Printf("processing %x...", res)
			var f frame
			var ok bool
			res, ok = decode(res)
			if !ok {
				// broken, need to get this frame again
				addr, ok := L.findAddrByPortName(got.Name)
				if !ok {
					// connection is dead
					log.Printf("connection %s is dead", got.Name)
					sendSystemStatusToApp(DISCONNECT, got.Name)
					L.kickDeadConn(got.Name)
					killRing()
					continue
				}
				f, err := newFrame(addr, L.myAddr, retFrame, nil)
				if err != nil {
					log.Printf("abnormal: cannot create retFrame: %s", err)
					continue
				}
				sendToPort(got.Name, f.Marshal())
				continue
			}
			if err := f.Unmarshal(res); err != nil {
				log.Printf("cannot unmarshal %x", res)
				continue
			}

			processFrame(&f, got.Name)
		} else {
			buf = append(buf, got.Data...)
		}
	}
}

func (l *layer) findAddrByPortName(name string) (byte, bool) {
	a, ok := l.conns[name]
	return a, ok
}

func (l *layer) findPortNameByAddr(addr byte) string {
	for pn, a := range l.conns {
		if addr == a {
			return pn
		}
	}

	return ""
}

func (l *layer) getRandomPortName() string {
	for pn := range l.conns {
		return pn
	}

	return ""
}

func (l *layer) getAnotherPort(addr string) string {
	for pn := range l.conns {
		if pn != addr {
			return pn
		}
	}

	return ""
}

func processFrame(f *frame, from string) {
	switch f.fType {
	case iFrame:
		// get message!
		if f.dest != L.myAddr { // this should not happen, because we have 3 computers
			// not my message, pass to the next
			port := L.getAnotherPort(from)
			if port == "" {
				log.Printf("disconnect, cannot find another port")
				sendSystemStatusToApp(DISCONNECT, L.lastDead)
				killRing()
				return
			}
			sendToPort(port, f.Marshal())
			return
		}

		// 3 computers, so we know all port names
		port := L.findPortNameByAddr(f.src)
		L.SendAppC <- &Action{
			AType: MessageType,
			Data: &MessageAction{
				Addr:    port,
				Message: f.data,
			},
		}
		if f.dest == broadcast {
			port := L.getAnotherPort(from)
			if port == "" {
				log.Printf("disconnect, cannot find another port")
				sendSystemStatusToApp(DISCONNECT, L.lastDead)
				killRing()
				return
			}
			sendToPort(port, f.Marshal())
		}
	case linkFrame:
		// set ring conns
		if f.src != L.myAddr {
			port := L.getAnotherPort(from)
			if port == "" {
				log.Printf("disconnect, cannot find another port")
				sendSystemStatusToApp(DISCONNECT, L.lastDead)
				return
			} else {
				newF, err := newFrame(0, f.src+1, linkFrame, nil)
				if err != nil {
					log.Printf("abnormal: new link frame err: %s", err)
					return
				}
				L.myAddr = f.src + 1
				sendToPort(port, newF.Marshal())
			}
		}
		sendSystemStatusToApp(CONNECT_RING, "OK")
	case uplinkFrame:
		if f.src != L.myAddr {
			port := L.getAnotherPort(from)
			if port == "" {
				log.Printf("disconnect, cannot find another port")
				sendSystemStatusToApp(DISCONNECT, L.lastDead)
			} else {
				sendToPort(port, f.Marshal())
			}
		}
		L.myAddr = 0
		sendSystemStatusToApp(DISRUPTION, "by frame")
	case ackFrame:
		// successful delivery
		L.delivered <- struct{}{}
		log.Printf("ACK, last frame %+x", lastFrame)
		sendSystemStatusToApp(ACK, "OK")
		lastFrame = nil
	case retFrame:
		// resend last frame
		sendToPort(from, lastFrame)
	default:
		// unknown frame
	}
}

func DisconnectByPortName(p string) {
	L.kickDeadConn(p)
	killRing()
	sendSystemStatusToApp(DISRUPTION, "by com-port")
}

func sendAnotherErrorToApp(format string, a ...interface{}) {
	L.SendAppC <- &Action{
		AType: SystemType,
		Data: SystemStatus{
			Status:  ANOTHER,
			Message: fmt.Sprintf(format, a),
		},
	}
}

func sendSystemStatusToApp(status byte, format string, a ...interface{}) {
	L.SendAppC <- &Action{
		AType: SystemType,
		Data: SystemStatus{
			Status:  status,
			Message: fmt.Sprintf(format, a),
		},
	}
}

func sendToPort(addr string, data []byte) {
	com.L.SendC <- &com.SendInfo{
		Name: addr,
		Data: data,
	}
}

func Init() {
	L = newLayer(queueLen)
	go L.listenToAppLayer()
	go L.listenToPhysLayer()
}

func Close() {
	close(L.SendAppC)
	close(L.GetAppC)
	close(L.delivered)
}
