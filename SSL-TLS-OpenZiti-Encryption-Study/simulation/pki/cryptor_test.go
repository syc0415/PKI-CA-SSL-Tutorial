package pki

import (
	"bytes"
	"crypto/rand"
	"simulation/config"
	"testing"

	"golang.org/x/crypto/chacha20poly1305"
)

func TestEncodeDecode(t *testing.T) {
	txHeader := make([]byte, config.STREAM_HEADER_SIZE)
	if _, err := rand.Read(txHeader); err != nil {
		t.Fatal(err)
	}
	common_test(t, NewEncryptor, NewDecryptor, txHeader)
}

func common_test(t *testing.T,
	makeEnc func([]byte, []byte) (Encryptor, error),
	makeDec func(k, h []byte) (Decryptor, error),
	txHeader []byte) {
	key, err := NewStreamKey()
	if err != nil {
		t.Fatal(err)
	}

	sender, err := makeEnc(key, txHeader)
	if err != nil {
		t.Fatal(err)
	}

	plain_text_messages := [][]byte{
		[]byte("Hello world"),
		randomData(100),
		randomData(1000),
		randomData(10000),
		[]byte("This is good-bye!"),
	}

	var coded_msgs [][]byte

	for i, m := range plain_text_messages {
		coded, err := sender.Push(m, byte(i%2))
		if err != nil {
			t.Error(err)
		}
		coded_msgs = append(coded_msgs, coded)
	}

	var decoded_msgs [][]byte
	receiver, err := makeDec(key, txHeader)
	if err != nil {
		t.Fatal(err)
	}

	for i, m := range coded_msgs {
		decoded, tag, err := receiver.Pull(m)
		if err != nil {
			t.Error("decoding error", err)
		}
		if tag != byte(i%2) {
			t.Errorf("unexpected tag received")
		}
		decoded_msgs = append(decoded_msgs, decoded)
	}

	for i := range plain_text_messages {
		if !bytes.Equal(plain_text_messages[i], decoded_msgs[i]) {
			t.Error("failed to decode")
		}
	}
}

func NewStreamKey() ([]byte, error) {
	k := make([]byte, chacha20poly1305.KeySize)
	_, err := rand.Read(k)
	if err != nil {
		return nil, err
	}
	return k, nil
}

func randomData(c int) []byte {
	out := make([]byte, c)
	if _, err := rand.Read(out); err != nil {
		panic(err)
	}
	return out
}
