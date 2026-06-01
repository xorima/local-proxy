package adapters

import (
	"crypto/des"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"strings"
	"unicode/utf16"
)

var (
	supportsUnicode   uint32 = 1
	requestTarget     uint32 = 4
	negotiateNTLM     uint32 = 0x200
	negotiateAlwaysSign uint32 = 0x8000
	negotiateNTLM2Key uint32 = 0x80000
	negotiateTargetInfo uint32 = 0x200000
	negotiate128        uint32 = 0x20000000
	negotiateKeyExchange uint32 = 0x40000000
	negotiate56          uint32 = 0x80000000
)

func defaultFlags() uint32 {
	return supportsUnicode | requestTarget | negotiateNTLM |
		negotiateAlwaysSign | negotiateNTLM2Key | negotiateTargetInfo |
		negotiate128 | negotiate56 | negotiateKeyExchange
}

func MD4(data []byte) []byte {
	return md4Hash(data)
}

func md4Hash(data []byte) []byte {
	// MD4 implementation per RFC 1320
	var buf [64]byte
	var h = [4]uint32{0x67452301, 0xefcdab89, 0x98badcfe, 0x10325476}

	// Pre-processing: append 0x80, then zeros, then length in bits
	msgLen := len(data)
	padLen := (55 - msgLen) % 64
	if padLen < 0 {
		padLen += 64
	}

	var padded []byte
	padded = append(padded, data...)
	padded = append(padded, 0x80)
	for i := 0; i < padLen; i++ {
		padded = append(padded, 0)
	}

	// Append length in bits (little-endian 64-bit)
	bitLen := uint64(msgLen) * 8
	padded = append(padded, byte(bitLen), byte(bitLen>>8), byte(bitLen>>16), byte(bitLen>>24),
		byte(bitLen>>32), byte(bitLen>>40), byte(bitLen>>48), byte(bitLen>>56))

	for i := 0; i < len(padded); i += 64 {
		copy(buf[:], padded[i:i+64])
		md4Transform(&h, &buf)
	}

	result := make([]byte, 16)
	binary.LittleEndian.PutUint32(result[0:4], h[0])
	binary.LittleEndian.PutUint32(result[4:8], h[1])
	binary.LittleEndian.PutUint32(result[8:12], h[2])
	binary.LittleEndian.PutUint32(result[12:16], h[3])
	return result
}

func md4Transform(h *[4]uint32, block *[64]byte) {
	var x [16]uint32
	for i := 0; i < 16; i++ {
		x[i] = binary.LittleEndian.Uint32(block[i*4 : i*4+4])
	}

	a, b, c, d := h[0], h[1], h[2], h[3]

	// Round 1
	md4Round1(&a, b, c, d, x[0], 3)
	md4Round1(&d, a, b, c, x[1], 7)
	md4Round1(&c, d, a, b, x[2], 11)
	md4Round1(&b, c, d, a, x[3], 19)
	md4Round1(&a, b, c, d, x[4], 3)
	md4Round1(&d, a, b, c, x[5], 7)
	md4Round1(&c, d, a, b, x[6], 11)
	md4Round1(&b, c, d, a, x[7], 19)
	md4Round1(&a, b, c, d, x[8], 3)
	md4Round1(&d, a, b, c, x[9], 7)
	md4Round1(&c, d, a, b, x[10], 11)
	md4Round1(&b, c, d, a, x[11], 19)
	md4Round1(&a, b, c, d, x[12], 3)
	md4Round1(&d, a, b, c, x[13], 7)
	md4Round1(&c, d, a, b, x[14], 11)
	md4Round1(&b, c, d, a, x[15], 19)

	// Round 2
	md4Round2(&a, b, c, d, x[0], 3)
	md4Round2(&d, a, b, c, x[4], 5)
	md4Round2(&c, d, a, b, x[8], 9)
	md4Round2(&b, c, d, a, x[12], 13)
	md4Round2(&a, b, c, d, x[1], 3)
	md4Round2(&d, a, b, c, x[5], 5)
	md4Round2(&c, d, a, b, x[9], 9)
	md4Round2(&b, c, d, a, x[13], 13)
	md4Round2(&a, b, c, d, x[2], 3)
	md4Round2(&d, a, b, c, x[6], 5)
	md4Round2(&c, d, a, b, x[10], 9)
	md4Round2(&b, c, d, a, x[14], 13)
	md4Round2(&a, b, c, d, x[3], 3)
	md4Round2(&d, a, b, c, x[7], 5)
	md4Round2(&c, d, a, b, x[11], 9)
	md4Round2(&b, c, d, a, x[15], 13)

	// Round 3
	md4Round3(&a, b, c, d, x[0], 3)
	md4Round3(&d, a, b, c, x[8], 9)
	md4Round3(&c, d, a, b, x[4], 11)
	md4Round3(&b, c, d, a, x[12], 15)
	md4Round3(&a, b, c, d, x[2], 3)
	md4Round3(&d, a, b, c, x[10], 9)
	md4Round3(&c, d, a, b, x[6], 11)
	md4Round3(&b, c, d, a, x[14], 15)
	md4Round3(&a, b, c, d, x[1], 3)
	md4Round3(&d, a, b, c, x[9], 9)
	md4Round3(&c, d, a, b, x[5], 11)
	md4Round3(&b, c, d, a, x[13], 15)
	md4Round3(&a, b, c, d, x[3], 3)
	md4Round3(&d, a, b, c, x[11], 9)
	md4Round3(&c, d, a, b, x[7], 11)
	md4Round3(&b, c, d, a, x[15], 15)

	h[0] += a
	h[1] += b
	h[2] += c
	h[3] += d
}

func md4Round1(a *uint32, b, c, d, x uint32, s uint) {
	*a += (b & c) | (^b & d) + x
	*a = (*a << s) | (*a >> (32 - s))
}

func md4Round2(a *uint32, b, c, d, x uint32, s uint) {
	*a += (b & c) | (b & d) | (c & d) + x + 0x5A827999
	*a = (*a << s) | (*a >> (32 - s))
}

func md4Round3(a *uint32, b, c, d, x uint32, s uint) {
	*a += b ^ c ^ d + x + 0x6ED9EBA1
	*a = (*a << s) | (*a >> (32 - s))
}

func utf16le(s string) []byte {
	runes := utf16.Encode([]rune(s))
	buf := make([]byte, len(runes)*2)
	for i, r := range runes {
		binary.LittleEndian.PutUint16(buf[i*2:], r)
	}
	return buf
}

func NTHash(password string) []byte {
	return md4Hash(utf16le(password))
}

func NTOWFv2(password, user, domain string) []byte {
	// NTOWFv2 = HMAC-MD5(NT hash, (user.toUpper() + domain) in UTF-16LE)
	ntHash := NTHash(password)
	data := utf16le(strings.ToUpper(user) + domain)

	mac := hmac.New(md5.New, ntHash)
	mac.Write(data)
	return mac.Sum(nil)
}

func GenerateType1(workstation, domain string) []byte {
	ws := utf16le(workstation)
	dom := utf16le(domain)

	flags := defaultFlags()

	headerLen := 32
	totalLen := headerLen + len(ws) + len(dom)
	msg := make([]byte, totalLen)

	copy(msg[0:8], []byte("NTLMSSP\x00"))
	msg[8] = 1

	binary.LittleEndian.PutUint32(msg[12:16], flags)

	// Domain security buffer at offset 16 (data starts at offset 32)
	domOffset := uint32(headerLen)
	writeSecurityBuffer(msg, 16, dom, domOffset)

	// Workstation security buffer at offset 24 (data starts after domain)
	wsOffset := domOffset + uint32(len(dom))
	writeSecurityBuffer(msg, 24, ws, wsOffset)

	// Copy payload data
	if len(dom) > 0 {
		copy(msg[domOffset:], dom)
	}
	if len(ws) > 0 {
		copy(msg[wsOffset:], ws)
	}

	return msg
}

func GenerateType2Challenge(serverNonce []byte) []byte {
	if len(serverNonce) != 8 {
		serverNonce = make([]byte, 8)
		copy(serverNonce, []byte("01234567"))
	}

	flags := defaultFlags()

	msg := make([]byte, 56)
	copy(msg[0:8], []byte("NTLMSSP\x00"))
	msg[8] = 2 // Type 2

	// Target name (empty security buffer)
	writeSecurityBuffer(msg, 12, nil, 56)

	// Flags
	binary.LittleEndian.PutUint32(msg[20:24], flags)

	// Challenge (8 bytes at offset 24)
	copy(msg[24:32], serverNonce)

	// Context (8 bytes at offset 32, all zeros)
	// Target info (empty security buffer at offset 40)
	writeSecurityBuffer(msg, 40, nil, 56)

	return msg
}

func GenerateType3V1(user, password, domain string, type2Msg []byte) []byte {
	if len(type2Msg) < 32 {
		return nil
	}
	challenge := type2Msg[24:32]
	ntHash := NTHash(password)
	lmResp := lmResponse(ntHash, challenge)
	ntResp := lmResponse(ntHash, challenge)
	return buildType3(user, domain, lmResp, ntResp)
}

func GenerateType3V2(user, password, domain string, type2Msg []byte) []byte {
	if len(type2Msg) < 32 {
		return nil
	}
	challenge := type2Msg[24:32]
	clientNonce := make([]byte, 8)
	rand.Read(clientNonce)
	ntowfv2 := NTOWFv2(password, user, domain)
	ntResp := buildNTLMv2Blob(ntowfv2, challenge, clientNonce)
	lmResp := make([]byte, 24)
	copy(lmResp, clientNonce)
	return buildType3(user, domain, lmResp, ntResp)
}

func GenerateType3Session(user, password, domain string, type2Msg []byte) []byte {
	if len(type2Msg) < 32 {
		return nil
	}
	serverChallenge := type2Msg[24:32]

	// NTLM2 Session Response:
	// 1. Client generates 8-byte nonce
	// 2. Mixed challenge = HMAC-MD5(serverChallenge + clientNonce)[0..7]
	// 3. LM Response = client nonce (8 bytes) + padding (16 bytes)
	// 4. NT Response = DES(ntHash[0..7], mixedChallenge) + DES(ntHash[7..14], mixedChallenge) + padding
	clientNonce := make([]byte, 8)
	rand.Read(clientNonce)

	mac := hmac.New(md5.New, serverChallenge)
	mac.Write(clientNonce)
	mixedChallenge := mac.Sum(nil)[:8]

	ntHash := NTHash(password)
	ntResp := lmResponse(ntHash, mixedChallenge)

	lmResp := make([]byte, 24)
	copy(lmResp, clientNonce)

	return buildType3(user, domain, lmResp, ntResp)
}

func buildType3(user, domain string, lmResp, ntResp []byte) []byte {
	u := utf16le(user)
	d := utf16le(domain)

	var offset uint32 = 64
	lmOffset := offset
	offset += uint32(len(lmResp))
	ntOffset := offset
	offset += uint32(len(ntResp))
	domOffset := offset
	offset += uint32(len(d))
	userOffset := offset
	offset += uint32(len(u))
	sessionOffset := offset

	totalLen := int(offset)
	msg := make([]byte, totalLen)

	copy(msg[0:8], []byte("NTLMSSP\x00"))
	msg[8] = 3

	writeSecurityBuffer(msg, 12, lmResp, lmOffset)
	writeSecurityBuffer(msg, 20, ntResp, ntOffset)
	writeSecurityBuffer(msg, 28, d, domOffset)
	writeSecurityBuffer(msg, 36, u, userOffset)
	writeSecurityBuffer(msg, 44, nil, sessionOffset)
	binary.LittleEndian.PutUint32(msg[52:56], defaultFlags())

	if len(lmResp) > 0 {
		copy(msg[lmOffset:lmOffset+uint32(len(lmResp))], lmResp)
	}
	if len(ntResp) > 0 {
		copy(msg[ntOffset:ntOffset+uint32(len(ntResp))], ntResp)
	}
	if len(d) > 0 {
		copy(msg[domOffset:domOffset+uint32(len(d))], d)
	}
	if len(u) > 0 {
		copy(msg[userOffset:userOffset+uint32(len(u))], u)
	}

	return msg[:totalLen]
}

func buildNTLMv2Blob(ntowfv2, serverChallenge, clientNonce []byte) []byte {
	// NTLMv2 Response = HMAC-MD5(ntowfv2, serverChallenge + blob)
	// blob = timestamp + clientNonce + padding + targetInfo + padding + flags

	timestamp := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestamp, uint64(0)) // timestamp (can be zero for testing)

	var blob []byte
	blob = append(blob, 0x01, 0x01) // response version
	blob = append(blob, 0x00, 0x00) // high part of version
	blob = append(blob, timestamp...) // timestamp
	blob = append(blob, clientNonce...) // client nonce
	blob = append(blob, 0x00, 0x00, 0x00, 0x00) // padding
	blob = append(blob, 0x00, 0x00, 0x00, 0x00) // no target info
	blob = append(blob, 0x00, 0x00, 0x00, 0x00) // no target info length

	// HMAC-MD5 of server challenge + blob
	mac := hmac.New(md5.New, ntowfv2)
	mac.Write(serverChallenge)
	mac.Write(blob)
	ntlmv2Response := mac.Sum(nil)

	// Full NTLMv2 response = HMAC(16 bytes) + blob
	result := make([]byte, 16+len(blob))
	copy(result[0:16], ntlmv2Response)
	copy(result[16:], blob)

	return result
}

func lmResponse(ntHash, challenge []byte) []byte {
	// LM response: DES(ntHash[0..7], challenge) + DES(ntHash[8..14], challenge)
	// For simplicity, use a basic DES-based implementation
	if len(ntHash) < 16 {
		ntHash = append(ntHash, make([]byte, 16-len(ntHash))...)
	}

	resp := make([]byte, 24)
	copy(resp, desEncrypt(ntHash[:7], challenge))
	copy(resp[8:], desEncrypt(ntHash[7:14], challenge))
	copy(resp[16:], make([]byte, 8))

	return resp
}

func desEncrypt(key7, data []byte) []byte {
	if len(key7) < 7 {
		key7 = append(key7, make([]byte, 7-len(key7))...)
	}
	// Convert 7-byte key to 8-byte DES key with parity
	key := make([]byte, 8)
	key[0] = key7[0]
	key[1] = (key7[0] << 7) | (key7[1] >> 1)
	key[2] = (key7[1] << 6) | (key7[2] >> 2)
	key[3] = (key7[2] << 5) | (key7[3] >> 3)
	key[4] = (key7[3] << 4) | (key7[4] >> 4)
	key[5] = (key7[4] << 3) | (key7[5] >> 5)
	key[6] = (key7[5] << 2) | (key7[6] >> 6)
	key[7] = key7[6] << 1

	// Simple DES ECB encryption using crypto/des
	block, err := des.NewCipher(key)
	if err != nil {
		return make([]byte, 8)
	}

	dst := make([]byte, 8)
	if len(data) < 8 {
		data = append(data, make([]byte, 8-len(data))...)
	}
	block.Encrypt(dst, data[:8])
	return dst
}

func ExtractChallenge(type2Msg []byte) []byte {
	if len(type2Msg) < 32 {
		return nil
	}
	nonce := make([]byte, 8)
	copy(nonce, type2Msg[24:32])
	return nonce
}

func NTLMHeader(user, password, domain string, type2Challenge []byte, ntlmv2 bool) string {
	if type2Challenge == nil {
		msg := GenerateType1("", domain)
		return "NTLM " + base64.StdEncoding.EncodeToString(msg)
	}

	var msg []byte
	if ntlmv2 {
		msg = GenerateType3V2(user, password, domain, type2Challenge)
	} else {
		msg = GenerateType3V1(user, password, domain, type2Challenge)
	}
	if msg == nil {
		return ""
	}
	return "NTLM " + base64.StdEncoding.EncodeToString(msg)
}

func EncodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func writeSecurityBuffer(msg []byte, offset int, data []byte, dataOffset uint32) {
	if len(data) > 0 {
		binary.LittleEndian.PutUint16(msg[offset:offset+2], uint16(len(data)))
		binary.LittleEndian.PutUint16(msg[offset+2:offset+4], uint16(len(data)))
		binary.LittleEndian.PutUint32(msg[offset+4:offset+8], dataOffset)
	} else {
		binary.LittleEndian.PutUint16(msg[offset:offset+2], 0)
		binary.LittleEndian.PutUint16(msg[offset+2:offset+4], 0)
		binary.LittleEndian.PutUint32(msg[offset+4:offset+8], dataOffset)
	}
}
