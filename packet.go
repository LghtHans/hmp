package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
)

// ---- Protocol constants ----

const (
	MagicNumber uint32 = 0x484D5032 // "HMP2"
	Version     uint8  = 0x02

	HeaderSize = 19 // bytes: 4+1+1+1+4+4+4
)

// Message types
const (
	MsgHello      uint8 = 0x01
	MsgHelloAck   uint8 = 0x02
	MsgText       uint8 = 0x03
	MsgAck        uint8 = 0x04
	MsgFileStart  uint8 = 0x05
	MsgFileChunk  uint8 = 0x06
	MsgFileEnd    uint8 = 0x07
	MsgPing       uint8 = 0x08
	MsgPong       uint8 = 0x09
	MsgBye        uint8 = 0x0A
	MsgOnline     uint8 = 0x0B
)

// Flag bits (bitfield, bit 0 = LSB)
const (
	FlagEncrypted   uint8 = 1 << 0
	FlagCompressed  uint8 = 1 << 1
	FlagFragmented  uint8 = 1 << 2
	FlagRequiresAck uint8 = 1 << 3
)

// ---- Errors ----

var (
	ErrPacketTooShort  = errors.New("hmp: packet shorter than header size")
	ErrBadMagic        = errors.New("hmp: invalid magic number")
	ErrUnsupportedVer  = errors.New("hmp: unsupported protocol version")
	ErrLengthMismatch  = errors.New("hmp: payload length does not match declared length")
	ErrChecksumFailed  = errors.New("hmp: checksum verification failed")
)

// ---- Packet struct ----

// Packet represents a single HMP protocol packet: fixed header + payload.
type Packet struct {
	Version     uint8
	MessageType uint8
	Flags       uint8
	Sequence    uint32
	Payload     []byte
}

// HasFlag reports whether the given flag bit is set.
func (p *Packet) HasFlag(flag uint8) bool {
	return p.Flags&flag != 0
}

// SetFlag sets the given flag bit.
func (p *Packet) SetFlag(flag uint8) {
	p.Flags |= flag
}

// NewPacket constructs a Packet with the current protocol version pre-filled.
func NewPacket(msgType uint8, seq uint32, payload []byte) *Packet {
	return &Packet{
		Version:     Version,
		MessageType: msgType,
		Flags:       0,
		Sequence:    seq,
		Payload:     payload,
	}
}

// Encode serializes the packet into wire format:
// [Magic(4)][Version(1)][MsgType(1)][Flags(1)][Sequence(4)][PayloadLen(4)][Checksum(4)][Payload...]
// All multi-byte fields are big-endian.
func (p *Packet) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)

	if err := binary.Write(buf, binary.BigEndian, MagicNumber); err != nil {
		return nil, err
	}
	if err := buf.WriteByte(p.Version); err != nil {
		return nil, err
	}
	if err := buf.WriteByte(p.MessageType); err != nil {
		return nil, err
	}
	if err := buf.WriteByte(p.Flags); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, p.Sequence); err != nil {
		return nil, err
	}

	payloadLen := uint32(len(p.Payload))
	if err := binary.Write(buf, binary.BigEndian, payloadLen); err != nil {
		return nil, err
	}

	checksum := crc32.ChecksumIEEE(p.Payload)
	if err := binary.Write(buf, binary.BigEndian, checksum); err != nil {
		return nil, err
	}

	if payloadLen > 0 {
		buf.Write(p.Payload)
	}

	return buf.Bytes(), nil
}

// Decode parses raw wire bytes into a Packet, validating magic number,
// version, declared payload length, and checksum.
func Decode(data []byte) (*Packet, error) {
	if len(data) < HeaderSize {
		return nil, ErrPacketTooShort
	}

	r := bytes.NewReader(data)

	var magic uint32
	if err := binary.Read(r, binary.BigEndian, &magic); err != nil {
		return nil, err
	}
	if magic != MagicNumber {
		return nil, ErrBadMagic
	}

	version, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	if version != Version {
		return nil, ErrUnsupportedVer
	}

	msgType, err := r.ReadByte()
	if err != nil {
		return nil, err
	}

	flags, err := r.ReadByte()
	if err != nil {
		return nil, err
	}

	var seq uint32
	if err := binary.Read(r, binary.BigEndian, &seq); err != nil {
		return nil, err
	}

	var payloadLen uint32
	if err := binary.Read(r, binary.BigEndian, &payloadLen); err != nil {
		return nil, err
	}

	var wantChecksum uint32
	if err := binary.Read(r, binary.BigEndian, &wantChecksum); err != nil {
		return nil, err
	}

	remaining := data[HeaderSize:]
	if uint32(len(remaining)) != payloadLen {
		return nil, ErrLengthMismatch
	}

	gotChecksum := crc32.ChecksumIEEE(remaining)
	if gotChecksum != wantChecksum {
		return nil, ErrChecksumFailed
	}

	payload := make([]byte, payloadLen)
	copy(payload, remaining)

	return &Packet{
		Version:     version,
		MessageType: msgType,
		Flags:       flags,
		Sequence:    seq,
		Payload:     payload,
	}, nil
}
