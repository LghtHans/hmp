package main

import (
	"bytes"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	original := NewPacket(MsgText, 42, []byte("hello, ray"))

	encoded, err := original.Encode()
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if decoded.Version != Version {
		t.Errorf("version mismatch: got %d want %d", decoded.Version, Version)
	}
	if decoded.MessageType != MsgText {
		t.Errorf("message type mismatch: got %d want %d", decoded.MessageType, MsgText)
	}
	if decoded.Sequence != 42 {
		t.Errorf("sequence mismatch: got %d want %d", decoded.Sequence, 42)
	}
	if !bytes.Equal(decoded.Payload, original.Payload) {
		t.Errorf("payload mismatch: got %q want %q", decoded.Payload, original.Payload)
	}
}

func TestEncodeEmptyPayload(t *testing.T) {
	p := NewPacket(MsgPing, 1, nil)
	encoded, err := p.Encode()
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	if len(encoded) != HeaderSize {
		t.Errorf("expected header-only length %d, got %d", HeaderSize, len(encoded))
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(decoded.Payload) != 0 {
		t.Errorf("expected empty payload, got %d bytes", len(decoded.Payload))
	}
}

func TestDecodeRejectsBadMagic(t *testing.T) {
	data := make([]byte, HeaderSize)
	_, err := Decode(data) // all zero bytes, wrong magic
	if err != ErrBadMagic {
		t.Errorf("expected ErrBadMagic, got %v", err)
	}
}

func TestDecodeRejectsShortPacket(t *testing.T) {
	_, err := Decode([]byte{0x01, 0x02})
	if err != ErrPacketTooShort {
		t.Errorf("expected ErrPacketTooShort, got %v", err)
	}
}

func TestDecodeRejectsCorruptedPayload(t *testing.T) {
	p := NewPacket(MsgText, 7, []byte("original data"))
	encoded, _ := p.Encode()

	// Flip a byte inside the payload region (after the 19-byte header)
	encoded[HeaderSize] ^= 0xFF

	_, err := Decode(encoded)
	if err != ErrChecksumFailed {
		t.Errorf("expected ErrChecksumFailed, got %v", err)
	}
}

func TestFlags(t *testing.T) {
	p := NewPacket(MsgFileChunk, 3, []byte("chunk"))
	p.SetFlag(FlagRequiresAck)

	if !p.HasFlag(FlagRequiresAck) {
		t.Error("expected FlagRequiresAck to be set")
	}
	if p.HasFlag(FlagEncrypted) {
		t.Error("did not expect FlagEncrypted to be set")
	}
}
