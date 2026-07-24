package source

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Source RCON wire format, little-endian throughout:
//
//	int32 length   // byte count of everything AFTER this field
//	int32 id       // caller-chosen; echoed back for correlation
//	int32 type     // see the packetType constants
//	byte[] body    // ASCII, NUL-terminated
//	byte  pad      // a second NUL terminating the (unused) trailing string
//
// The length prefix excludes itself, so the minimum legal value is 10:
// 4 (id) + 4 (type) + 1 (empty body terminator) + 1 (pad).
type packetType int32

const (
	// typeResponseValue is a server's reply to a command. It is also the type
	// we send for the sentinel packet used to detect the end of a multi-packet
	// response -- see client.Execute.
	typeResponseValue packetType = 0

	// typeExecCommand asks the server to run a command.
	//
	// Deliberately the same value as typeAuthResponse (2). The protocol
	// overloads 2 in both directions and disambiguates by who is speaking:
	// client->server 2 means EXECCOMMAND, server->client 2 means
	// AUTH_RESPONSE. Since we only ever decode server->client packets, a
	// decoded 2 is always an auth response.
	typeExecCommand packetType = 2

	// typeAuthResponse is the server's verdict on an auth attempt.
	typeAuthResponse packetType = 2

	// typeAuth submits the RCON password.
	typeAuth packetType = 3
)

func (t packetType) String() string {
	switch t {
	case typeResponseValue:
		return "RESPONSE_VALUE"
	case typeExecCommand: // == typeAuthResponse
		return "EXECCOMMAND/AUTH_RESPONSE"
	case typeAuth:
		return "AUTH"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", int32(t))
	}
}

const (
	// headerSize is everything the length prefix covers except the body:
	// id + type + body terminator + pad.
	headerSize = 4 + 4 + 1 + 1

	// maxBodySize is the largest body a server will send in one packet.
	// Longer replies arrive split across several packets.
	maxBodySize = 4096

	// maxPacketSize caps what we are willing to allocate from a length prefix.
	// A malicious or desynchronised peer must not be able to make us allocate
	// arbitrary memory. The slack over maxBodySize tolerates servers that are
	// slightly more generous than the reference implementation.
	maxPacketSize = headerSize + maxBodySize*2

	// authFailedID is the id a server echoes on AUTH_RESPONSE to reject a
	// password. This is the *only* signal of auth failure -- there is no error
	// body and the connection is not necessarily closed.
	authFailedID int32 = -1
)

// errShortPacket means the length prefix was smaller than a header can be.
var errShortPacket = errors.New("source: packet shorter than header")

type packet struct {
	id   int32
	typ  packetType
	body string
}

// encode serialises p including its length prefix.
func (p packet) encode() []byte {
	length := headerSize + len(p.body)

	buf := make([]byte, 0, 4+length)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(length))
	buf = binary.LittleEndian.AppendUint32(buf, uint32(p.id))
	buf = binary.LittleEndian.AppendUint32(buf, uint32(p.typ))
	buf = append(buf, p.body...)
	buf = append(buf, 0, 0) // body terminator + pad
	return buf
}

// decodePacket reads exactly one packet from r.
func decodePacket(r io.Reader) (packet, error) {
	var lengthBuf [4]byte
	if _, err := io.ReadFull(r, lengthBuf[:]); err != nil {
		return packet{}, err
	}
	length := int32(binary.LittleEndian.Uint32(lengthBuf[:]))

	if length < headerSize {
		return packet{}, fmt.Errorf("%w: got %d, need >= %d", errShortPacket, length, headerSize)
	}
	if length > maxPacketSize {
		return packet{}, fmt.Errorf("source: packet length %d exceeds maximum %d", length, maxPacketSize)
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return packet{}, err
	}

	p := packet{
		id:  int32(binary.LittleEndian.Uint32(payload[0:4])),
		typ: packetType(int32(binary.LittleEndian.Uint32(payload[4:8]))),
	}

	// Trim trailing NULs rather than assuming exactly two. The reference
	// implementation sends body-terminator + pad, but some servers send only
	// one, and trimming handles both without a special case.
	p.body = decodeBody(bytes.TrimRight(payload[8:], "\x00"))
	return p, nil
}

// decodeBody converts a raw body to a Go string.
//
// The protocol nominally specifies ASCII, but Minecraft servers send the colour
// code prefix as byte 0xA7 (SECTION SIGN in ISO-8859-1), which is not valid
// UTF-8. Converting such a body with a plain string() leaves invalid bytes that
// later get replaced with U+FFFD -- notably by encoding/json, so colour codes
// would arrive at the UI mangled.
//
// Modern servers do send UTF-8, so neither charset can simply be assumed.
// Valid UTF-8 is passed through untouched (which covers all pure ASCII), and
// anything else is read as ISO-8859-1, where every byte maps to the code point
// of the same value.
func decodeBody(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}

	var sb strings.Builder
	sb.Grow(len(b))
	for _, c := range b {
		sb.WriteRune(rune(c))
	}
	return sb.String()
}
