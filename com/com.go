package com

import (
	"errors"
	"log"
	"path"
	"regexp"

	"github.com/tarm/serial"
)

const (
	DefaultBaudRate = 115200
)

var (
	devPath = "/dev"                        // Unix path to devices
	comName = "ttyS"                        // Linux com-ports
	portNum = regexp.MustCompile("[0-9]+$") // pass ttyS0, com0 or 0

	conns = make(map[string]*Port, 2)

	ErrConnNotFound = errors.New("connection not found")
)

type Port struct {
	p   *serial.Port
	cfg *Config
}

type Config struct {
	Name     string `json:"name"`
	BaudRate int    `json:"baudRate"`
	Size     byte   `json:"size"`
	Parity   string `json:"parity"`
	StopBits byte   `json:"stopBits"`
}

func Connect(cfg *Config) error {
	var parity serial.Parity
	switch cfg.Parity {
	case "odd":
		parity = serial.ParityOdd
	case "even":
		parity = serial.ParityEven
	case "mark":
		parity = serial.ParityMark
	case "space":
		parity = serial.ParitySpace
	default:
		parity = serial.ParityNone
	}
	ms := portNum.FindStringSubmatch(cfg.Name)
	if len(ms) == 0 {
		return errors.New("cannot parse port num")
	}
	num := ms[0]
	c := &serial.Config{
		Name:     path.Join(devPath, comName+num),
		Baud:     cfg.BaudRate,
		Size:     cfg.Size,
		Parity:   parity,
		StopBits: serial.StopBits(cfg.StopBits),
	}
	s, err := serial.OpenPort(c)
	if err != nil {
		return err
	}

	p := &Port{
		p:   s,
		cfg: cfg,
	}
	conns[cfg.Name] = p

	go listenPort(p)

	return nil
}

func ClosePort(addr string) error {
	if c, ok := conns[addr]; ok {
		delete(conns, addr)
		return c.p.Close()
	} else {
		return ErrConnNotFound
	}
}

func write(addr string, b []byte) error {
	if c, ok := conns[addr]; ok {
		_, err := c.p.Write(b)
		return err
	} else {
		return ErrConnNotFound
	}
}

func listenPort(s *Port) {
	var buf []byte
	for {
		_, err := s.p.Read(buf)
		if err != nil {
			// todo: error
			log.Printf("com: listen port %s err: %s", s.cfg.Name, err)
			continue // or inform data link layer (port is dead)
		}
		log.Printf("got chunk: %x", buf)

		// send chunks to the data link layer
		res := make([]byte, len(buf))
		copy(res, buf)
		log.Printf("sending chunk to data link layer: %x", res)
		L.GotC <- &SendInfo{
			Name: s.cfg.Name,
			Data: res,
		}
	}
}
