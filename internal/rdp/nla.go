package rdp

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/asn1"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/imrx44/redbrut/internal/classifier"
)

// RDP negotiation request — asks server to use NLA (CredSSP).
var rdpNegReq = []byte{
	0x03, 0x00, 0x00, 0x13, // TPKT header: version=3, len=19
	0x0e,                   // X.224 length
	0xe0,                   // X.224 CR TPDU
	0x00, 0x00,             // dst-ref
	0x00, 0x00,             // src-ref
	0x00,                   // class
	// RDP Negotiation Request
	0x01,                   // type=RDP_NEG_REQ
	0x00,                   // flags
	0x08, 0x00,             // length=8
	0x03, 0x00, 0x00, 0x00, // requestedProtocols: PROTOCOL_SSL|PROTOCOL_HYBRID
}

// AttemptNLA performs a full NLA (CredSSP/NTLM) RDP authentication attempt.
// It returns the classified result based on the NTSTATUS code from the server.
func AttemptNLA(ctx context.Context, host string, port int, username, password string, timeout time.Duration) classifier.AuthResult {
	addr := fmt.Sprintf("%s:%d", host, port)

	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return classifier.ResultNetworkError
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(timeout))

	// Step 1: RDP negotiation — request NLA
	if _, err := conn.Write(rdpNegReq); err != nil {
		return classifier.ResultNetworkError
	}

	resp := make([]byte, 19)
	if _, err := readFull(conn, resp); err != nil {
		return classifier.ResultNetworkError
	}

	// Check negotiation response — byte 11 is type: 0x02=RDP_NEG_RSP, 0x03=RDP_NEG_FAILURE
	if len(resp) < 12 {
		return classifier.ResultProtocolError
	}
	negType := resp[11]
	if negType == 0x03 {
		// Server refused NLA — could be Classic RDP only
		return classifier.ResultProtocolError
	}

	// Step 2: Upgrade to TLS
	tlsConn := tls.Client(conn, &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS10, // RDP servers often use TLS 1.0/1.1
	})
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return classifier.ResultNetworkError
	}

	// Step 3: CredSSP/NTLM exchange
	return performCredSSP(ctx, tlsConn, username, password, host)
}

// performCredSSP runs the CredSSP (SPNEGO/NTLM) exchange over a TLS connection.
func performCredSSP(ctx context.Context, conn *tls.Conn, username, password, host string) classifier.AuthResult {
	// NTLM Negotiate message
	negotiateMsg := buildNTLMNegotiate()
	negoToken := wrapSPNEGONegTokenInit(negotiateMsg)
	tsReq1 := buildTSRequest(1, [][]byte{negoToken}, nil)

	if _, err := conn.Write(tsReq1); err != nil {
		return classifier.ResultNetworkError
	}

	// Read server TSRequest with NTLM Challenge
	buf, err := readTSRequest(conn)
	if err != nil {
		return classifier.ResultNetworkError
	}

	challengeMsg, err := extractNTLMFromTSRequest(buf)
	if err != nil {
		return classifier.ResultProtocolError
	}

	// Parse NTLM Challenge
	challenge, err := parseNTLMChallenge(challengeMsg)
	if err != nil {
		return classifier.ResultProtocolError
	}

	// Build NTLM Authenticate with NTLMv2
	// Domain and username split from "DOMAIN\user" or just "user"
	domain, user := splitDomain(username)
	authenticateMsg, err := buildNTLMAuthenticate(challenge, user, domain, password, host)
	if err != nil {
		return classifier.ResultProtocolError
	}

	authToken := wrapSPNEGONegTokenResp(authenticateMsg)
	tsReq2 := buildTSRequest(3, [][]byte{authToken}, nil)

	if _, err := conn.Write(tsReq2); err != nil {
		return classifier.ResultNetworkError
	}

	// Read final TSRequest — contains NTSTATUS or pubKeyAuth
	finalBuf, err := readTSRequest(conn)
	if err != nil {
		// Connection closed immediately often means auth failed
		if isEOF(err) {
			return classifier.ResultInvalid
		}
		return classifier.ResultNetworkError
	}

	status, err := extractNTStatusFromTSRequest(finalBuf)
	if err != nil {
		// If we can't parse but got a response, treat as unknown (retry)
		return classifier.ResultUnknown
	}

	return classifier.FromNTStatus(status)
}

// --- NTLM message builders ---

const (
	ntlmNegotiateFlags uint32 = 0x62088215 // NTLMSSP standard negotiate flags
)

func buildNTLMNegotiate() []byte {
	// NTLM Negotiate message (Type 1)
	buf := &bytes.Buffer{}
	buf.WriteString("NTLMSSP\x00")
	binary.Write(buf, binary.LittleEndian, uint32(1)) // MessageType = Negotiate
	binary.Write(buf, binary.LittleEndian, ntlmNegotiateFlags)
	// Domain and Workstation fields (empty)
	buf.Write(bytes.Repeat([]byte{0}, 16)) // DomainNameFields + WorkstationFields
	// Version (optional but some servers require it)
	buf.Write([]byte{0x0a, 0x00, 0x63, 0x45, 0x00, 0x00, 0x00, 0x0f})
	return buf.Bytes()
}

type ntlmChallenge struct {
	ServerChallenge [8]byte
	Flags           uint32
	TargetName      string
	TargetInfo      []byte
}

func parseNTLMChallenge(msg []byte) (*ntlmChallenge, error) {
	if len(msg) < 56 {
		return nil, fmt.Errorf("challenge too short: %d", len(msg))
	}
	if !bytes.HasPrefix(msg, []byte("NTLMSSP\x00")) {
		return nil, fmt.Errorf("not NTLM message")
	}
	msgType := binary.LittleEndian.Uint32(msg[8:12])
	if msgType != 2 {
		return nil, fmt.Errorf("expected type 2, got %d", msgType)
	}

	c := &ntlmChallenge{}
	copy(c.ServerChallenge[:], msg[24:32])
	c.Flags = binary.LittleEndian.Uint32(msg[20:24])

	// TargetInfo
	tiLen := binary.LittleEndian.Uint16(msg[40:42])
	tiOffset := binary.LittleEndian.Uint32(msg[44:48])
	if int(tiOffset)+int(tiLen) <= len(msg) {
		c.TargetInfo = msg[tiOffset : tiOffset+uint32(tiLen)]
	}

	return c, nil
}

func buildNTLMAuthenticate(challenge *ntlmChallenge, username, domain, password, workstation string) ([]byte, error) {
	// NTLMv2 response
	clientChallenge := make([]byte, 8)
	for i := range clientChallenge {
		clientChallenge[i] = byte(rand.Intn(256))
	}

	ntResponse, sessionKey, err := computeNTLMv2(
		password, username, domain,
		challenge.ServerChallenge[:], clientChallenge,
		challenge.TargetInfo,
	)
	if err != nil {
		return nil, err
	}
	_ = sessionKey

	userUTF16 := encodeUTF16LE(strings.ToUpper(username))
	domainUTF16 := encodeUTF16LE(strings.ToUpper(domain))
	wsUTF16 := encodeUTF16LE(workstation)

	// Layout: fixed header (72 bytes) + variable data
	lmRespOffset := uint32(72)
	ntRespOffset := lmRespOffset + 24 // LM response is 24 bytes (placeholder)
	domainOffset := ntRespOffset + uint32(len(ntResponse))
	userOffset := domainOffset + uint32(len(domainUTF16))
	wsOffset := userOffset + uint32(len(userUTF16))
	sessionKeyOffset := wsOffset + uint32(len(wsUTF16))

	buf := &bytes.Buffer{}
	buf.WriteString("NTLMSSP\x00")
	binary.Write(buf, binary.LittleEndian, uint32(3)) // MessageType = Authenticate

	// LmChallengeResponseFields
	writeSecBuf(buf, 24, lmRespOffset)
	// NtChallengeResponseFields
	writeSecBuf(buf, uint16(len(ntResponse)), ntRespOffset)
	// DomainNameFields
	writeSecBuf(buf, uint16(len(domainUTF16)), domainOffset)
	// UserNameFields
	writeSecBuf(buf, uint16(len(userUTF16)), userOffset)
	// WorkstationFields
	writeSecBuf(buf, uint16(len(wsUTF16)), wsOffset)
	// EncryptedRandomSessionKeyFields
	writeSecBuf(buf, 0, sessionKeyOffset)
	// NegotiateFlags
	binary.Write(buf, binary.LittleEndian, ntlmNegotiateFlags)
	// Version
	buf.Write([]byte{0x0a, 0x00, 0x63, 0x45, 0x00, 0x00, 0x00, 0x0f})
	// MIC (16 zero bytes — we skip MIC computation for speed)
	buf.Write(bytes.Repeat([]byte{0}, 16))

	// Payload
	buf.Write(bytes.Repeat([]byte{0}, 24)) // LM response placeholder
	buf.Write(ntResponse)
	buf.Write(domainUTF16)
	buf.Write(userUTF16)
	buf.Write(wsUTF16)

	return buf.Bytes(), nil
}

func writeSecBuf(buf *bytes.Buffer, length uint16, offset uint32) {
	binary.Write(buf, binary.LittleEndian, length)  // Len
	binary.Write(buf, binary.LittleEndian, length)  // MaxLen
	binary.Write(buf, binary.LittleEndian, offset)  // Offset
}

// --- SPNEGO / CredSSP wrappers ---

// OID for NTLMSSP SPNEGO
var ntlmOID = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 2, 2, 10}

func wrapSPNEGONegTokenInit(ntlmToken []byte) []byte {
	// NegTokenInit: mechTypes=[NTLMSSP], mechToken=ntlmToken
	mechTypeBytes, _ := asn1.Marshal([]asn1.ObjectIdentifier{ntlmOID})
	tokenSeq, _ := asn1.Marshal(asn1.RawValue{
		Class:       asn1.ClassContextSpecific,
		Tag:         2,
		IsCompound:  false,
		Bytes:       ntlmToken,
	})
	initSeq, _ := asn1.Marshal(asn1.RawValue{
		Class:      asn1.ClassUniversal,
		Tag:        asn1.TagSequence,
		IsCompound: true,
		Bytes: append(
			asn1MarshalExplicit(0, mechTypeBytes),
			asn1MarshalExplicit(2, tokenSeq)...,
		),
	})
	// Wrap in SPNEGO OID application tag
	spnegoOID := asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 2}
	oidBytes, _ := asn1.Marshal(spnegoOID)
	appBytes := append(oidBytes, initSeq...)
	appWrapped, _ := asn1.Marshal(asn1.RawValue{
		Class:      asn1.ClassApplication,
		Tag:        0,
		IsCompound: true,
		Bytes:      appBytes,
	})
	return appWrapped
}

func wrapSPNEGONegTokenResp(ntlmToken []byte) []byte {
	// NegTokenResp: negState=accept-incomplete, responseToken=ntlmToken
	stateBytes, _ := asn1.Marshal(asn1.Enumerated(1)) // accept-incomplete
	tokenBytes, _ := asn1.Marshal(asn1.RawValue{
		Class: asn1.ClassContextSpecific, Tag: 2,
		IsCompound: false, Bytes: ntlmToken,
	})
	seq, _ := asn1.Marshal(asn1.RawValue{
		Class: asn1.ClassUniversal, Tag: asn1.TagSequence, IsCompound: true,
		Bytes: append(
			asn1MarshalExplicit(0, stateBytes),
			asn1MarshalExplicit(2, tokenBytes)...,
		),
	})
	resp, _ := asn1.Marshal(asn1.RawValue{
		Class: asn1.ClassContextSpecific, Tag: 1, IsCompound: true,
		Bytes: seq,
	})
	return resp
}

func asn1MarshalExplicit(tag int, inner []byte) []byte {
	r, _ := asn1.Marshal(asn1.RawValue{
		Class: asn1.ClassContextSpecific, Tag: tag,
		IsCompound: true, Bytes: inner,
	})
	return r
}

// --- TSRequest (CredSSP outer container) ---

// buildTSRequest wraps tokens in a CredSSP TSRequest ASN.1 structure.
func buildTSRequest(version int, negoTokens [][]byte, authInfo []byte) []byte {
	// TSRequest ::= SEQUENCE {
	//   version    [0] INTEGER,
	//   negoTokens [1] NegoData OPTIONAL,
	//   authInfo   [4] OCTET STRING OPTIONAL,
	// }
	versionBytes, _ := asn1.Marshal(version)
	versionField := asn1MarshalExplicit(0, versionBytes)

	var body []byte
	body = append(body, versionField...)

	if len(negoTokens) > 0 {
		var tokenSeqs []byte
		for _, tok := range negoTokens {
			tokField, _ := asn1.Marshal(asn1.RawValue{
				Class: asn1.ClassContextSpecific, Tag: 0,
				IsCompound: false, Bytes: tok,
			})
			tokSeq, _ := asn1.Marshal(asn1.RawValue{
				Class: asn1.ClassUniversal, Tag: asn1.TagSequence,
				IsCompound: true, Bytes: tokField,
			})
			tokenSeqs = append(tokenSeqs, tokSeq...)
		}
		seqOf, _ := asn1.Marshal(asn1.RawValue{
			Class: asn1.ClassUniversal, Tag: asn1.TagSequence,
			IsCompound: true, Bytes: tokenSeqs,
		})
		body = append(body, asn1MarshalExplicit(1, seqOf)...)
	}

	if authInfo != nil {
		aiBytes, _ := asn1.Marshal(authInfo)
		body = append(body, asn1MarshalExplicit(4, aiBytes)...)
	}

	tsReq, _ := asn1.Marshal(asn1.RawValue{
		Class: asn1.ClassUniversal, Tag: asn1.TagSequence,
		IsCompound: true, Bytes: body,
	})
	return tsReq
}

// readTSRequest reads a length-prefixed TSRequest from the connection.
func readTSRequest(conn net.Conn) ([]byte, error) {
	// TSRequest is ASN.1 DER — first byte is tag (0x30 = SEQUENCE),
	// then length in BER encoding.
	header := make([]byte, 4)
	if _, err := readFull(conn, header[:2]); err != nil {
		return nil, err
	}
	if header[0] != 0x30 {
		return nil, fmt.Errorf("expected SEQUENCE tag 0x30, got 0x%02x", header[0])
	}

	var totalLen int
	var headerLen int
	if header[1]&0x80 == 0 {
		totalLen = int(header[1])
		headerLen = 2
	} else {
		numBytes := int(header[1] & 0x7f)
		if numBytes > 3 {
			return nil, fmt.Errorf("ASN.1 length too large")
		}
		if _, err := readFull(conn, header[2:2+numBytes]); err != nil {
			return nil, err
		}
		for i := 0; i < numBytes; i++ {
			totalLen = totalLen<<8 | int(header[2+i])
		}
		headerLen = 2 + numBytes
	}

	body := make([]byte, totalLen)
	if _, err := readFull(conn, body); err != nil {
		return nil, err
	}

	full := make([]byte, headerLen+totalLen)
	copy(full, header[:headerLen])
	copy(full[headerLen:], body)
	return full, nil
}

// extractNTLMFromTSRequest extracts the NTLM token from a server TSRequest.
func extractNTLMFromTSRequest(data []byte) ([]byte, error) {
	// Walk ASN.1 structure to find the negoTokens field [1]
	// and extract the inner NTLM bytes.
	var outer asn1.RawValue
	rest, err := asn1.Unmarshal(data, &outer)
	if err != nil || len(rest) > 0 {
		return nil, fmt.Errorf("unmarshal TSRequest: %w", err)
	}

	remaining := outer.Bytes
	for len(remaining) > 0 {
		var field asn1.RawValue
		remaining, err = asn1.Unmarshal(remaining, &field)
		if err != nil {
			break
		}
		if field.Class == asn1.ClassContextSpecific && field.Tag == 1 {
			// negoTokens — dig into SEQUENCE OF NegoDataItem
			return extractTokenBytes(field.Bytes), nil
		}
	}
	return nil, fmt.Errorf("negoTokens field not found")
}

func extractTokenBytes(data []byte) []byte {
	// SEQUENCE OF → SEQUENCE → [0] OCTET STRING
	var v asn1.RawValue
	rest, err := asn1.Unmarshal(data, &v)
	if err != nil || len(rest) > 0 {
		return nil
	}
	rest = v.Bytes
	for len(rest) > 0 {
		var item asn1.RawValue
		rest, err = asn1.Unmarshal(rest, &item)
		if err != nil {
			break
		}
		if item.Class == asn1.ClassUniversal && item.Tag == asn1.TagSequence {
			inner := item.Bytes
			var tok asn1.RawValue
			if _, err := asn1.Unmarshal(inner, &tok); err == nil {
				if tok.Class == asn1.ClassContextSpecific && tok.Tag == 0 {
					return tok.Bytes
				}
			}
		}
	}
	return nil
}

// extractNTStatusFromTSRequest extracts the NTSTATUS from the final server TSRequest.
//
// On SUCCESS the server sends pubKeyAuth [3] — no errorCode field at all.
// On FAILURE the server sends errorCode [4] containing the NTSTATUS.
func extractNTStatusFromTSRequest(data []byte) (uint32, error) {
	var outer asn1.RawValue
	_, err := asn1.Unmarshal(data, &outer)
	if err != nil {
		return 0, err
	}

	remaining := outer.Bytes
	for len(remaining) > 0 {
		var field asn1.RawValue
		remaining, err = asn1.Unmarshal(remaining, &field)
		if err != nil {
			break
		}
		if field.Class != asn1.ClassContextSpecific {
			continue
		}
		switch field.Tag {
		case 3:
			// pubKeyAuth present → server accepted our credentials → SUCCESS
			return classifier.NTStatusSuccess, nil
		case 4:
			// errorCode → auth failed, extract NTSTATUS
			var codeVal asn1.RawValue
			if _, err := asn1.Unmarshal(field.Bytes, &codeVal); err == nil {
				if len(codeVal.Bytes) >= 4 {
					code := binary.BigEndian.Uint32(codeVal.Bytes[len(codeVal.Bytes)-4:])
					return code, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("neither pubKeyAuth nor errorCode found in TSRequest")
}

// --- helpers ---

func encodeUTF16LE(s string) []byte {
	// Convert UTF-8 string to UTF-16LE bytes — critical for non-ASCII passwords.
	// This correctly handles Russian (Cyrillic), Chinese, Arabic, etc.
	encoded := utf16.Encode([]rune(s))
	buf := make([]byte, len(encoded)*2)
	for i, r := range encoded {
		binary.LittleEndian.PutUint16(buf[i*2:], r)
	}
	return buf
}

func splitDomain(username string) (domain, user string) {
	if idx := strings.Index(username, "\\"); idx != -1 {
		return username[:idx], username[idx+1:]
	}
	if idx := strings.Index(username, "@"); idx != -1 {
		return username[idx+1:], username[:idx]
	}
	return "", username
}

func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func isEOF(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "EOF") || strings.Contains(s, "connection reset")
}
