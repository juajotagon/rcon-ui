package source

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestPacketRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		p    packet
	}{
		{"empty body", packet{id: 1, typ: typeAuth}},
		{"simple", packet{id: 42, typ: typeExecCommand, body: "list"}},
		{"auth failure id", packet{id: authFailedID, typ: typeAuthResponse}},
		{"max body", packet{id: 7, typ: typeResponseValue, body: strings.Repeat("x", maxBodySize)}},
		{"utf8 body", packet{id: 9, typ: typeResponseValue, body: "café ☃"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodePacket(bytes.NewReader(tc.p.encode()))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got != tc.p {
				t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, tc.p)
			}
		})
	}
}

func TestEncodeLengthExcludesItself(t *testing.T) {
	p := packet{id: 1, typ: typeExecCommand, body: "list"}
	buf := p.encode()

	length := binary.LittleEndian.Uint32(buf[:4])
	if want := uint32(headerSize + len("list")); length != want {
		t.Errorf("length prefix = %d, want %d", length, want)
	}
	// The prefix counts every byte after itself.
	if int(length) != len(buf)-4 {
		t.Errorf("length %d does not match %d trailing bytes", length, len(buf)-4)
	}
}

func TestDecodeRejectsUndersizedLength(t *testing.T) {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint32(headerSize-1))
	buf.Write(make([]byte, headerSize))

	if _, err := decodePacket(&buf); !errors.Is(err, errShortPacket) {
		t.Errorf("err = %v, want errShortPacket", err)
	}
}

// A desynchronised or hostile peer must not be able to make us allocate an
// arbitrary buffer from an attacker-controlled length prefix.
func TestDecodeRejectsOversizedLength(t *testing.T) {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint32(1<<30))

	_, err := decodePacket(&buf)
	if err == nil || !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("err = %v, want an oversize rejection", err)
	}
}

func TestDecodeTruncatedPayload(t *testing.T) {
	p := packet{id: 1, typ: typeResponseValue, body: "hello"}
	full := p.encode()

	_, err := decodePacket(bytes.NewReader(full[:len(full)-3]))
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("err = %v, want io.ErrUnexpectedEOF", err)
	}
}

// The reference implementation terminates the body and then pads with a second
// NUL, but some servers send only one. Trimming handles both.
func TestDecodeToleratesSingleTerminator(t *testing.T) {
	body := "hello"
	length := 4 + 4 + len(body) + 1 // one NUL only

	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint32(length))
	binary.Write(&buf, binary.LittleEndian, uint32(3))
	binary.Write(&buf, binary.LittleEndian, uint32(typeResponseValue))
	buf.WriteString(body)
	buf.WriteByte(0)

	got, err := decodePacket(&buf)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.body != body {
		t.Errorf("body = %q, want %q", got.body, body)
	}
}
