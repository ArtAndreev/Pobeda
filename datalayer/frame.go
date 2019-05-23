package datalayer

import (
	"bytes"
	"errors"
	"fmt"
	"log"
)

const (
	startByte byte = 0xFF
	stopByte  byte = 0xFF

	maxDataLen       = 1<<8 - 1 // 255 bytes, because len field is byte
	minFrameLen      = 5        // 5 bytes
	minAddr     byte = 0x01
	maxAddr     byte = 0x7E
	broadcast   byte = 0x7F
)

var (
	ErrWrongFrame   = errors.New("frame is wrong")
	ErrDataTooLarge = fmt.Errorf("data len exceeds %d bytes", maxDataLen)
)

// fTypes
const (
	iFrame      = iota // data frame
	linkFrame          // init ring
	linkOKFrame        // broadcast it after successful init of ring
	uplinkFrame        // kill ring
	ackFrame           // got frame is ok, send ok
	retFrame           // got frame is not ok, ask for this frame again
)

type frame struct {
	start byte
	dest  byte
	src   byte
	fType byte
	len   byte   // optional
	data  []byte // optional
	stop  byte
}

func newFrame(dest, src, fType byte, data []byte) (*frame, error) {
	if len(data) > maxDataLen {
		return nil, ErrDataTooLarge
	}
	d := make([]byte, len(data))
	copy(d, data)
	f := &frame{
		start: startByte,
		dest:  dest,
		src:   src,
		fType: fType,
		len:   byte(len(data)),
		data:  d,
		stop:  stopByte,
	}

	return f, nil
}

func (f *frame) Marshal() []byte {
	var b []byte
	b = append(b, f.start, f.dest, f.src, f.fType, f.len)
	b = append(b, f.data...)
	b = append(b, f.stop)

	return b
}

func (f *frame) Unmarshal(v []byte) error {
	if len(v) < minFrameLen || v[0] != startByte || v[len(v)-1] != stopByte {
		return ErrWrongFrame
	}
	f.start = v[0]
	f.dest = v[1]
	f.src = v[2]
	f.fType = v[3]
	f.len = v[4]
	if f.len != 0 {
		f.data = v[5 : 5+f.len]
	}
	f.stop = v[5+f.len]

	if int(5+f.len+1) != len(v) {
		log.Printf("incorrect read of frame")
		return ErrWrongFrame
	}

	return nil
}

func isValidFrame(f []byte) bool {
	return f[0] == startByte && f[len(f)-1] == stopByte
}

func hasStartByte(f []byte) bool {
	return f[0] == startByte
}

func findEndOfFrame(d []byte) int {
	return bytes.Index(d, []byte{stopByte})
}
