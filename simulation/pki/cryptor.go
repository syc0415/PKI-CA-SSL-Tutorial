package pki

import (
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"fmt"
	"simulation/config"

	"golang.org/x/crypto/chacha20"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/poly1305"
)

var pad0 [16]byte

type streamState struct {
	k     [config.STREAM_KEY_SIZE]byte
	nonce [chacha20poly1305.NonceSize]byte
	pad   [8]byte
}

func (s *streamState) reset() {
	for i := range s.nonce {
		s.nonce[i] = 0
	}
	s.nonce[0] = 1
}

type Encryptor interface {
	Push([]byte, byte) ([]byte, error)
}

type Decryptor interface {
	Pull([]byte) ([]byte, byte, error)
}

func NewEncryptor(key, header []byte) (Encryptor, error) {
	if len(key) != config.STREAM_KEY_SIZE {
		return nil, errors.New("failed to new encryptor:\n\tkey length is expected to be " + fmt.Sprint(config.STREAM_KEY_SIZE) + " but " + fmt.Sprint(len(key)))
	}

	stream := &encryptor{}
	k, err := chacha20.HChaCha20(key, header[:config.CRYPTO_CORE_HCHACHA20_INPUTSIZE])
	if err != nil {
		return nil, errors.New("failed to new encryptor:\n\t" + err.Error())
	}
	copy(stream.k[:], k)
	stream.reset()

	copy(stream.nonce[config.CRYPTO_SECRETSTREAM_XCHACHA20POLY1305_COUNTERBYTES:], header[config.CRYPTO_CORE_HCHACHA20_INPUTSIZE:])

	copy(stream.pad[:], pad0[:])

	return stream, nil
}

func NewDecryptor(key, header []byte) (Decryptor, error) {
	stream := &decryptor{}
	k, err := chacha20.HChaCha20(key, header[:config.CRYPTO_CORE_HCHACHA20_INPUTSIZE])
	if err != nil {
		return nil, errors.New("failed to new decryptor:\n\t" + err.Error())
	}
	copy(stream.k[:], k)
	stream.reset()

	copy(stream.nonce[config.CRYPTO_SECRETSTREAM_XCHACHA20POLY1305_COUNTERBYTES:], header[config.CRYPTO_CORE_HCHACHA20_INPUTSIZE:])

	copy(stream.pad[:], pad0[:])

	return stream, nil
}

type encryptor struct {
	streamState
}

func (e *encryptor) Push(plain []byte, tag byte) ([]byte, error) {
	mlen := len(plain)
	out := make([]byte, mlen+config.STREAM_A_SIZE)

	chacha, err := chacha20.NewUnauthenticatedCipher(e.k[:], e.nonce[:])
	if err != nil {
		return nil, errors.New("failed to push:\n\t" + err.Error())
	}

	var block [64]byte
	chacha.XORKeyStream(block[:], block[:])

	var polyInit [32]byte
	copy(polyInit[:], block[:])
	poly := poly1305.New(&polyInit)

	memzero(block[:])
	block[0] = tag

	chacha.XORKeyStream(block[:], block[:])
	_, _ = poly.Write(block[:])
	out[0] = block[0]

	c := out[1:]
	chacha.XORKeyStream(c, plain)
	_, _ = poly.Write(c[:mlen])
	padIen := (0x10 - len(block) + mlen) & 0xf
	_, _ = poly.Write(pad0[:padIen])

	var slen [8]byte
	binary.LittleEndian.PutUint64(slen[:], uint64(0))
	_, _ = poly.Write(slen[:])

	binary.LittleEndian.PutUint64(slen[:], uint64(len(block)+mlen))
	_, _ = poly.Write(slen[:])

	mac := c[mlen:]
	copy(mac, poly.Sum(nil))

	xor_buf(e.nonce[config.CRYPTO_SECRETSTREAM_XCHACHA20POLY1305_COUNTERBYTES:], mac)
	buf_inc(e.nonce[:config.CRYPTO_SECRETSTREAM_XCHACHA20POLY1305_COUNTERBYTES])

	return out, nil
}

type decryptor struct {
	streamState
}

func (d *decryptor) Pull(in []byte) ([]byte, byte, error) {
	inlen := len(in)

	if inlen < config.STREAM_A_SIZE {
		return nil, 0, errors.New("failed to pull:\n\t input length expected to be " + fmt.Sprintf("%v", config.STREAM_A_SIZE) + " but " + fmt.Sprintf("%v", len(in)))
	}

	mlen := inlen - config.STREAM_A_SIZE

	chacha, err := chacha20.NewUnauthenticatedCipher(d.k[:], d.nonce[:])
	if err != nil {
		return nil, 0, errors.New("failed to pull:\n\t" + err.Error())
	}

	var block [64]byte
	chacha.XORKeyStream(block[:], block[:])

	var polyInit [32]byte
	copy(polyInit[:], block[:])
	poly := poly1305.New(&polyInit)

	memzero(block[:])
	block[0] = in[0]

	chacha.XORKeyStream(block[:], block[:])
	tag := block[0]
	block[0] = in[0]
	if _, err = poly.Write(block[:]); err != nil {
		return nil, 0, errors.New("failed to pull:\n\t" + err.Error())
	}

	c := in[1:]
	if _, err = poly.Write(c[:mlen]); err != nil {
		return nil, 0, errors.New("failed to pull:\n\t" + err.Error())
	}

	padlen := (0x10 - len(block) + mlen) & 0xf
	if _, err = poly.Write(pad0[:padlen]); err != nil {
		return nil, 0, errors.New("failed to pull:\n\t" + err.Error())
	}

	var slen [8]byte
	binary.LittleEndian.PutUint64(slen[:], uint64(0))
	if _, err = poly.Write(slen[:]); err != nil {
		return nil, 0, err
	}

	binary.LittleEndian.PutUint64(slen[:], uint64(len(block)+mlen))
	if _, err = poly.Write(slen[:]); err != nil {
		return nil, 0, err
	}

	mac := poly.Sum(nil)
	stored_mac := c[mlen:]
	if subtle.ConstantTimeCompare(mac, stored_mac) == 0 {
		return nil, 0, errors.New("failed to pull:\n\tsubtle constant time cmp fail:\n\tmac:" + string(mac) + "\n\tstored_mac:" + string(stored_mac))
	}
	m := make([]byte, mlen)
	chacha.XORKeyStream(m, c[:mlen])

	xor_buf(d.nonce[config.CRYPTO_SECRETSTREAM_XCHACHA20POLY1305_COUNTERBYTES:], mac)
	buf_inc(d.nonce[:config.CRYPTO_SECRETSTREAM_XCHACHA20POLY1305_COUNTERBYTES])

	return m, tag, nil
}

func memzero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func xor_buf(out, in []byte) {
	for i := range out {
		out[i] ^= in[i]
	}
}

func buf_inc(n []byte) {
	c := 1

	for i := range n {
		c += int(n[i])
		n[i] = byte(c)
		c >>= 8
	}
}
