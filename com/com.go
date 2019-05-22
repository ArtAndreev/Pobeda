package com

import (
	"errors"
	"io"
	"log"
	"path"
	"regexp"

	"github.com/jacobsa/go-serial/serial"
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
	p   io.ReadWriteCloser
	cfg *Config
}

type Config struct {
	Name     string `json:"name"`
	BaudRate uint   `json:"baudRate"`
	Size     uint   `json:"size"`
	Parity   string `json:"parity"`
	StopBits uint   `json:"stopBits"`
}

func Connect(cfg *Config) error {
	var parity serial.ParityMode
	switch cfg.Parity {
	case "odd":
		parity = serial.PARITY_ODD
	case "even":
		parity = serial.PARITY_EVEN
	default:
		parity = serial.PARITY_NONE
	}
	ms := portNum.FindStringSubmatch(cfg.Name)
	if len(ms) == 0 {
		return errors.New("cannot parse port num")
	}
	num := ms[0]
	if cfg.StopBits == 0 {
		cfg.StopBits = 1 // use default
	}
	c := serial.OpenOptions{
		PortName:        path.Join(devPath, comName+num),
		BaudRate:        cfg.BaudRate,
		DataBits:        cfg.Size,
		ParityMode:      parity,
		StopBits:        cfg.StopBits,
		MinimumReadSize: 4,
	}
	s, err := serial.Open(c)
	if err != nil {
		log.Printf("physical layer: error opening port '%s' with cfg %+v: %s", cfg.Name, cfg, err)
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
			if err == io.EOF {
				// ignore, nothing important
				continue
			}
			log.Printf("com: listen port %s err: %s", s.cfg.Name, err)
			continue // or inform data link layer (port is dead)
		}
		if len(buf) == 0 {
			continue
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
