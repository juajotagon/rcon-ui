package source

import (
	"bytes"
	"testing"
)

// Golden vectors pinning the exact wire bytes.
//
// Why these exist, given the round-trip tests: decode(encode(p)) == p passes
// even if the encoder is wrong, because the decoder is wrong in the same way.
// Swap the id and type fields in both, or flip both to big-endian, and every
// other test in this package still goes green -- while a real server rejects
// the packets. The two fake servers do not help either; they were written from
// the same understanding as the codec, so they replicate a misreading rather
// than catching it.
//
// The byte sequences below are therefore NOT produced by encode(). They are
// computed by hand from the published field table:
//
//	Length  int32  little-endian, counts everything after itself
//	ID      int32  little-endian
//	Type    int32  little-endian
//	Payload []byte NUL-terminated ASCII
//	Padding byte   one further NUL
//
// Sources: the Minecraft wiki's RCON page (https://minecraft.wiki/w/RCON),
// which documents the field order, little-endian integers, the 3/2/0 type
// values, and the id == -1 auth-failure convention; cross-checked against the
// Valve Source RCON protocol description.
//
// The arithmetic is spelled out per case so a reviewer can verify these
// without running any of this code.

func TestGoldenEncodeAuthPacket(t *testing.T) {
	// id 1, type 3 (AUTH), body "passwrd" (7 bytes).
	// Length = 4 (id) + 4 (type) + 7 (body) + 1 (terminator) + 1 (pad) = 17 = 0x11.
	//
	// This case is the one that catches a field-order swap: id (1) and type (3)
	// differ, so exchanging them changes the bytes.
	want := []byte{
		0x11, 0x00, 0x00, 0x00, // length 17, little-endian
		0x01, 0x00, 0x00, 0x00, // id 1
		0x03, 0x00, 0x00, 0x00, // type 3 = SERVERDATA_AUTH
		'p', 'a', 's', 's', 'w', 'r', 'd', // body
		0x00, // body terminator
		0x00, // padding
	}

	got := packet{id: 1, typ: typeAuth, body: "passwrd"}.encode()
	if !bytes.Equal(got, want) {
		t.Errorf("auth packet bytes differ\n got % x\nwant % x", got, want)
	}
	if len(got) != 21 {
		t.Errorf("total packet = %d bytes, want 21 (4 length + 17 counted)", len(got))
	}
}

func TestGoldenEncodeExecCommandPacket(t *testing.T) {
	// id 2, type 2 (EXECCOMMAND), body "list" (4 bytes).
	// Length = 4 + 4 + 4 + 1 + 1 = 14 = 0x0E.
	want := []byte{
		0x0e, 0x00, 0x00, 0x00, // length 14
		0x02, 0x00, 0x00, 0x00, // id 2
		0x02, 0x00, 0x00, 0x00, // type 2 = SERVERDATA_EXECCOMMAND
		'l', 'i', 's', 't',
		0x00,
		0x00,
	}

	got := packet{id: 2, typ: typeExecCommand, body: "list"}.encode()
	if !bytes.Equal(got, want) {
		t.Errorf("exec packet bytes differ\n got % x\nwant % x", got, want)
	}
}

// The auth-failure sentinel is the only signal a password was rejected, so its
// encoding is worth pinning: -1 as a little-endian int32 is four 0xFF bytes.
func TestGoldenAuthFailureResponse(t *testing.T) {
	// id -1, type 2 (AUTH_RESPONSE), empty body.
	// Length = 4 + 4 + 0 + 1 + 1 = 10 = 0x0A, the minimum legal packet.
	raw := []byte{
		0x0a, 0x00, 0x00, 0x00, // length 10
		0xff, 0xff, 0xff, 0xff, // id -1, two's complement
		0x02, 0x00, 0x00, 0x00, // type 2 = SERVERDATA_AUTH_RESPONSE
		0x00,
		0x00,
	}

	if got := (packet{id: authFailedID, typ: typeAuthResponse}).encode(); !bytes.Equal(got, raw) {
		t.Errorf("auth failure bytes differ\n got % x\nwant % x", got, raw)
	}

	// And decoding those exact bytes must recover the sentinel, since that is
	// what Dial keys ErrAuthFailed on.
	p, err := decodePacket(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.id != authFailedID {
		t.Errorf("id = %d, want %d", p.id, authFailedID)
	}
	if p.typ != typeAuthResponse {
		t.Errorf("type = %d, want %d", p.typ, typeAuthResponse)
	}
}

// Decoding a server-produced byte sequence, rather than our own output.
func TestGoldenDecodeResponseValue(t *testing.T) {
	// id 7, type 0 (RESPONSE_VALUE), body "Hi" (2 bytes).
	// Length = 4 + 4 + 2 + 1 + 1 = 12 = 0x0C.
	raw := []byte{
		0x0c, 0x00, 0x00, 0x00,
		0x07, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, // type 0 = SERVERDATA_RESPONSE_VALUE
		'H', 'i',
		0x00,
		0x00,
	}

	p, err := decodePacket(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := packet{id: 7, typ: typeResponseValue, body: "Hi"}
	if p != want {
		t.Errorf("got %+v, want %+v", p, want)
	}
}

// Minecraft sends its colour-code prefix as byte 0xA7 (ISO-8859-1 SECTION
// SIGN), which is not valid UTF-8. Left raw, it survives as an invalid string
// and is later replaced with U+FFFD -- by encoding/json among others -- so
// colour codes would reach the UI mangled.
func TestDecodeLatin1ColourCodes(t *testing.T) {
	// Body: 0xA7 'a' "Green" -- the classic "§a" colour prefix.
	raw := []byte{
		0x11, 0x00, 0x00, 0x00, // length 17 = 4 + 4 + 7 + 1 + 1
		0x01, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0xa7, 'a', 'G', 'r', 'e', 'e', 'n',
		0x00,
		0x00,
	}

	p, err := decodePacket(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if want := "§aGreen"; p.body != want {
		t.Errorf("body = %q, want %q", p.body, want)
	}
}

// A body that is already valid UTF-8 must pass through untouched, rather than
// being re-read as ISO-8859-1 and mojibaked.
func TestDecodeUTF8BodyUnchanged(t *testing.T) {
	body := "café ☃ 日本語"
	p, err := decodePacket(bytes.NewReader(packet{id: 1, typ: typeResponseValue, body: body}.encode()))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.body != body {
		t.Errorf("body = %q, want %q", p.body, body)
	}
}
