package datalayer

import (
	"errors"
	"fmt"
	"log"
	"time"

	"Pobeda/com"
)

const (
	queueLen = 32
	linkWait = 5 * time.Second
	sendWait = 5 * time.Second
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
	tempAddr  byte
	conns     map[string]byte
	lastDead  string // for messages from another peer to another peer (1 -> me ...dc... 3)
	connected chan struct{}
	delivered chan struct{}
}

func newLayer(len int) layer {
	return layer{
		SendAppC: make(chan *Action, len),
		GetAppC:  make(chan *Action, len),
		QueueLen: len,

		myAddr:    0,
		tempAddr:  0,
		conns:     make(map[string]byte, 2),
		connected: make(chan struct{}),
		delivered: make(chan struct{}),
	}
}

func (l *layer) listenToAppLayer() {
	for a := range l.SendAppC {
		sa, ok := a.Data.(SystemAction)
		if !ok {
			log.Printf("cannot cast to SystemAction '%T'", sa)
			continue
		}

		switch a.AType {
		case OP_CONNECT:
			if _, ok := L.conns[sa.Cfg.Name]; ok {
				log.Printf("cannot connect to %s: already connected", sa.Cfg.Name)
				SendActionStatusToApp(ERROR, sa.Cfg.Name, "", ErrPhysConnect)
				continue
			}
			if sa.Cfg == nil {
				log.Printf("cannot connect: no cfg available")
				SendActionStatusToApp(ERROR, "", "", ErrProtocolBug)
				continue
			}
			if err := com.Connect(sa.Cfg); err != nil {
				log.Printf("cannot connect to %s: %s", sa.Cfg.Name, err)
				SendActionStatusToApp(ERROR, sa.Cfg.Name, "", ErrPhysConnect)
				continue
			}
			L.conns[sa.Cfg.Name] = 0 // no addr => no logical connection
			log.Printf("connected to %s", sa.Cfg.Name)
			SendActionStatusToApp(CONNECT, sa.Cfg.Name, "", "")
		case OP_DISCONNECT:
			// disconnect gracefully killing the ring
			if L.myAddr != 0 {
				killRing()
			}

			if err := com.ClosePort(sa.Addr); err != nil {
				log.Printf("cannot disconnect from %s: %s", sa.Addr, err)
				// SendActionStatusToApp(ERROR,"cannot disconnect from %s: %s", sa.Addr, err)
				continue
			}
			log.Printf("successful disconnect from %s", sa.Addr)
			SendActionStatusToApp(DISCONNECT, sa.Addr, "", "")
			L.kickDeadConn(sa.Addr)
		case OP_RING_CONNECT:
			if L.myAddr == 0 {
				port := L.getRandomPortName()
				if port == "" {
					SendActionStatusToApp(ERROR, "", "", ErrRingConnect)
					continue
				}
				firstAddr := minAddr // init our addr only after successful receiving this frame back
				f, err := newFrame(broadcast, firstAddr, linkFrame, []byte{firstAddr})
				if err != nil {
					log.Printf("cannot ring connect: create link frame: %s", err)
					SendActionStatusToApp(ERROR, "", "", ErrRingConnect)
					continue
				}
				L.tempAddr = firstAddr
				sendToPort(port, f.Marshal())

				// wait for our frame back
				t := time.NewTimer(linkWait)
				select {
				case <-t.C:
					log.Printf("initiator: cannot ring connect: timeout")
					L.myAddr = 0
					SendActionStatusToApp(ERROR, "", "", ErrRingConnect)
				case <-L.connected:
					// inform other, that ring is closed
					f, err := newFrame(broadcast, minAddr, linkOKFrame, nil)
					if err != nil {
						log.Printf("cannot ring connect: create link ok frame: %s", err)
						SendActionStatusToApp(ERROR, "", "", ErrRingConnect)
						continue
					}
					sendToPort(port, f.Marshal())
					L.myAddr = firstAddr
					L.conns[port] = firstAddr + 1 // was for: next will have incremented addr
					log.Printf("CONNECT_RING: 'OK', myAddr is %d, neighbors are %+v", L.myAddr, L.conns)
					SendActionStatusToApp(CONNECT_RING, "OK", "", "")
				}
				t.Stop()
			} else {
				log.Printf("cannot ring connect: already connected")
				SendActionStatusToApp(ERROR, "", "", ErrRingConnect)
			}
		case OP_KILL_RING:
			killRing()
		case OP_SEND:
			ma, ok := a.Data.(SystemAction)
			if !ok {
				log.Printf("cannot cast to SystemAction '%T'", ma)
				continue
			}
			log.Printf("processing send of message: %+v", ma)
			var addr byte
			if ma.Addr != "" { // not broadcast
				var ok bool
				addr, ok = L.findAddrByPortName(ma.Addr)
				if !ok {
					log.Printf("cannot send message to %s", ma.Addr)
					// sendSystemStatusToApp(NO_ACK, "disconnected")
					// sendSystemStatusToApp(DISCONNECT, ma.Addr)
					L.kickDeadConn(ma.Addr)
					killRing()
					continue
				}
			} else {
				addr = broadcast
			}
			f, err := newFrame(addr, L.myAddr, iFrame, []byte(ma.Message))
			if err != nil {
				log.Printf("cannot put message to frame: %s", err)
				// sendAnotherErrorToApp("cannot put message to frame: %s", err)
				continue
			}
			if addr != broadcast {
				sendToPort(ma.Addr, f.Marshal())
				// let's wait for delivery
				t := time.NewTimer(sendWait)
				select {
				case <-t.C:
					log.Printf("fail to send")
					SendActionStatusToApp(NO_ACK, "", "", "")
				case <-l.delivered:
				}
				lastFrame = nil
			} else {
				port := L.getRandomPortName()
				if port == "" {
					// todo: disconnect
					log.Printf("cannot ring disconnect: no port available")
					// SendActionStatusToApp("cannot ring disconnect: no port available")
					SendActionStatusToApp(NO_ACK, "", "", "")
					continue
				}
				sendToPort(port, f.Marshal())
				SendActionStatusToApp(ACK, "", "", "") // broadcast is ok
			}
		default:
			log.Printf("unknown action type %d", a.AType)
			SendActionStatusToApp(ERROR, "", "", ErrProtocolBug)
		}
	}
}

func killRing() {
	port := L.getRandomPortName()
	if port == "" {
		log.Printf("cannot ring disconnect: no port available")
		// SendActionStatusToApp("cannot ring disconnect: no port available")
		return
	}
	if L.myAddr != 0 {
		f, err := newFrame(0, L.myAddr, uplinkFrame, nil)
		if err != nil {
			log.Printf("cannot ring disconnect: %s", err)
			// sendAnotherErrorToApp("cannot ring disconnect: %s", err)
			return
		}
		L.myAddr = 0
		sendToPort(port, f.Marshal())
		for k := range L.conns {
			L.conns[k] = 0
		}
	} else {
		log.Printf("cannot ring disconnect: already disconnected")
		// SendActionStatusToApp(ERROR, "")
	}

	SendActionStatusToApp(DISRUPTION, "", "", "")
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
			if hasStartByte(got.Data) {
				started = true
			} else {
				// broken, skip
				log.Printf("not valid frame: %+v", got)
				continue
			}
		}
		buf = append(buf, got.Data...)
		if len(buf) < minFrameLen {
			// skip, we need more
			log.Printf("not enough data (<%d), get next: %x", minFrameLen, got.Data)
			continue
		}

		// fType, err := getFrameType(buf)
		// if err != nil {
		// 	log.Printf("get frame type err: %s", err)
		// 	// buf = nil?
		// 	continue
		// }
		// frameSize := 0
		// switch fType {
		// case iFrame: // has len
		// 	if len(buf) < minFrameLen+1 /* need len field */ || len(buf) < minFrameLen+1+int(buf[4]) /* len */ {
		// 		// buf has not enough data, read more
		// 		continue
		// 	}
		// 	frameSize = minFrameLen + 1 + int(buf[4])
		// case linkFrame: // has len (=1)
		// 	// if len(buf) < minFrameLen + 1 {
		// 	//
		// 	// }
		// case linkOKFrame, uplinkFrame, ackFrame, retFrame:
		// 	//
		// default:
		// 	// unknown
		// 	// buf = nil?
		// 	continue
		// }

		// if fType != iFrame {
		if end := findEndOfFrame(buf[1:]); end != -1 { // finds first occurrence of stopByte
			// we searched without first byte, real end:
			end++
			// gotcha frame
			res := make([]byte, end+1) // fixme
			copy(res, buf)

			buf = []byte{}
			if end+1 < len(buf) {
				buf = append(buf, buf[end+1:]...)
			}
			if !isValidFrame(res) {
				log.Printf("abnormal: frame should be valid: %v", res)
			}

			log.Printf("processing %x...", res)
			// var ok bool
			// res, ok = decode(res)
			// if !ok {
			// 	// broken, need to get this frame again
			// 	addr, ok := L.findAddrByPortName(got.Name)
			// 	if !ok {
			// 		// connection is dead
			// 		log.Printf("connection %s is dead", got.Name)
			// 		SendActionStatusToApp(DISCONNECT, got.Name, "", "")
			// 		L.kickDeadConn(got.Name)
			// 		killRing()
			// 		continue
			// 	}
			// 	f, err := newFrame(addr, L.myAddr, retFrame, nil)
			// 	if err != nil {
			// 		log.Printf("abnormal: cannot create retFrame: %s", err)
			// 		continue
			// 	}
			// 	sendToPort(got.Name, f.Marshal())
			// 	continue
			// }

			var f frame
			if err := f.Unmarshal(res); err != nil {
				log.Printf("cannot unmarshal %x", res)
				continue
			}

			started = false
			processFrame(&f, got.Name)
		} else {
			// broken?
			log.Printf("not end, get next for: %x", got.Data)
		}
		// } else {
		// 	if end := buf[frameSize]; end != stopByte {
		// 		log.Printf("wrong frame, end byte is not stop")
		// 		buf = buf[frameSize:]
		// 		continue
		// 	}
		// 	var f frame
		// 	if err := f.Unmarshal(buf[:frameSize]); err != nil {
		// 		log.Printf("cannot unmarshal %x", buf[:frameSize])
		// 		continue
		// 	}
		//
		// 	started = false
		// 	processFrame(&f, got.Name)
		// }
	}
}

func getFrameType(b []byte) (byte, error) {
	if len(b) < minFrameLen {
		return 0, errors.New("frame has not enough len")
	}

	return b[3], nil
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
	log.Printf("processing frame %+v from %s...", f, from)
	switch f.fType {
	case iFrame:
		// get message!
		log.Printf("message frame: %+v, active ports: %+v", f, L.conns)
		if f.dest == broadcast {
			if f.src == L.myAddr {
				return
			}
			port := L.getAnotherPort(from)
			if port == "" {
				log.Printf("broadcast message: disconnect, cannot find another port")
				SendActionStatusToApp(DISCONNECT, L.lastDead, "", "")
				killRing()
				return
			}
			sendToPort(port, f.Marshal())
			if f.src == L.myAddr {
				return
			}
		} else if f.dest != L.myAddr { // this should not happen, because we have 3 computers
			// not my message, pass to the next
			port := L.getAnotherPort(from)
			if port == "" {
				log.Printf("not my message: disconnect, cannot find another port")
				SendActionStatusToApp(DISCONNECT, L.lastDead, "", "")
				killRing()
				return
			}
			sendToPort(port, f.Marshal())
			return
		}

		// 3 computers, so we know all port names
		port := L.findPortNameByAddr(f.src)
		if port == "" {
			log.Printf("my message: disconnect, cannot find another port")
			SendActionStatusToApp(DISCONNECT, L.lastDead, "", "")
			killRing()
			return
		}
		// L.SendAppC <- &Action{
		// 	AType: OP_SEND,
		// 	Data: SystemAction{
		// 		Addr:    port,
		// 		Message: string(f.data),
		// 	},
		// }
		if f.dest != broadcast {
			SendActionStatusToApp(MESSAGE, port, "not_broadcast", string(f.data))
		} else {
			SendActionStatusToApp(MESSAGE, port, "", string(f.data))
		}
	case linkFrame:
		// set ring conns
		if f.len != 1 {
			log.Printf("got strange link frame (len != 1): %+v", f)
			return
		}
		if L.myAddr == 0 {
			if f.src != L.tempAddr {
				port := L.getAnotherPort(from)
				if port == "" {
					log.Printf("disconnect, cannot find another port")
					SendActionStatusToApp(DISCONNECT, L.lastDead, "", "")
					return
				}

				newF, err := newFrame(broadcast, f.src, linkFrame, []byte{f.data[0] + 1})
				if err != nil {
					log.Printf("abnormal: new link frame err: %s", err)
					return
				}
				sendToPort(port, newF.Marshal())

				// wait in another goroutine, because link ok frame comes to the same channel as this link frame
				go func() {
					t := time.NewTimer(linkWait)
					select {
					case <-t.C:
						log.Printf("not initiator: cannot ring connect: timeout")
						SendActionStatusToApp(ERROR, "", "", ErrRingConnect)
					case <-L.connected:
						L.conns[from] = f.data[0]
						L.myAddr = f.data[0] + 1
						if f.data[0]+2 != 4 { // let's think there are 3 computers, need to fix
							L.conns[port] = f.data[0] + 2 // was for: next will have incremented addr
						} else {
							L.conns[port] = minAddr
						}
						log.Printf("CONNECT_RING: 'OK', myAddr is %d, neighbors are %+v", L.myAddr, L.conns)
						SendActionStatusToApp(CONNECT_RING, "OK", "", "")
					}
					t.Stop()
				}()
			} else {
				// we got frame back, logical conn is ok
				select {
				case L.connected <- struct{}{}:
					log.Println("link frame: got back")
					L.conns[from] = f.data[0] // we've already checked data len
				default:
					log.Println("abnormal: we got link frame with our src, but we don't listen for it...")
				}
			}
		} else {
			log.Println("got link frame, but already connected")
		}
	case linkOKFrame:
		if L.myAddr == 0 {
			select {
			case L.connected <- struct{}{}:
				log.Println("link ok frame: success")
			default:
				log.Println("abnormal: we got link ok frame, but we don't listen for it...")
			}
			// broadcast: pass the frame anyway
			port := L.getAnotherPort(from)
			if port == "" {
				log.Printf("disconnect, cannot find another port")
				SendActionStatusToApp(DISCONNECT, L.lastDead, "", "")
				return
			}
			sendToPort(port, f.Marshal())
		} else {
			log.Println("got link ok frame, but already connected")
		}
	case uplinkFrame:
		if L.myAddr != 0 {
			if f.src != L.myAddr {
				port := L.getAnotherPort(from)
				if port == "" {
					log.Printf("disconnect, cannot find another port")
					SendActionStatusToApp(DISCONNECT, L.lastDead, "", "")
				} else {
					sendToPort(port, f.Marshal())
				}
			}
			log.Println("")
			L.myAddr = 0
			SendActionStatusToApp(DISRUPTION, "", "", "")
		} else {
			log.Printf("got uplink back")
		}
	case ackFrame:
		// successful delivery
		L.delivered <- struct{}{}
		log.Printf("ACK, last frame %+x", lastFrame)
		// from: f.src, to: f.dest
		var to string
		if f.dest != 0 {
			to = L.findPortNameByAddr(f.dest)
		}
		SendActionStatusToApp(ACK, "", to, "")
		lastFrame = nil
	case retFrame:
		// resend last frame
		log.Printf("RET, last frame %+x", lastFrame)
		sendToPort(from, lastFrame)
	default:
		// unknown frame
	}
}

func SendActionStatusToApp(op byte, addr, messageTo, messageFormat string, a ...interface{}) {
	L.GetAppC <- &Action{
		AType: op,
		Data: ActionPayload{
			Addr:    addr,
			Message: fmt.Sprintf(messageFormat, a...),
			To:      messageTo,
		},
	}
}

func GetActionStatusFromApp(op byte, addr string, cfg *com.Config, message string) {
	L.SendAppC <- &Action{
		AType: op,
		Data: SystemAction{
			Addr:    addr,
			Cfg:     cfg,
			Message: message,
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
