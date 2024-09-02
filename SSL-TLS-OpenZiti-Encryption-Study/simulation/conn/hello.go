package conn

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"log"
	"net"
	"simulation/config"
	"time"
)

type request struct {
	Hello     string            `json:"hello"`
	PublicKey ed25519.PublicKey `json:"public_key"`
	Options   map[string][]byte `json:"options"`
}

func NewRequest(hello string, publicKey ed25519.PublicKey, options map[string][]byte) (*request, error) {
	if hello != config.CLIENT_HELLO {
		return nil, errors.New("failed to new request:\n\thello message expected to be CLIENT_HELLO")
	}
	if len(publicKey) != config.X25519_KEY_SIZE {
		return nil, errors.New("failed to new request:\n\tpublicKey is not a valid X25519 public key")
	}
	return &request{
		Hello:     hello,
		PublicKey: publicKey,
		Options:   options,
	}, nil
}

func (r *request) SendForReply(conn net.Conn) (*reply, error) {
	jreq, err := json.Marshal(r)
	if err != nil {
		return nil, errors.New("failed to marshell request:\n\t" + err.Error())
	}

	if err = conn.SetWriteDeadline(time.Now().Add(config.CONN_TIMEOUT)); err != nil {
		return nil, errors.New("failed to set conn write timeout:\n\t" + err.Error())
	}
	if _, err = conn.Write(jreq); err != nil {
		return nil, errors.New("failed to write to conn:\n\t" + err.Error())
	}
	log.Println(config.SENT, config.CLIENT_HELLO)
	if err = conn.SetWriteDeadline(time.Time{}); err != nil {
		return nil, errors.New("failed to reset write timeout:\n\t" + err.Error())
	}

	if err = conn.SetReadDeadline(time.Now().Add(config.CONN_TIMEOUT)); err != nil {
		return nil, errors.New("failed to set conn read timeout:\n\t" + err.Error())
	}
	var rep reply
	if err = json.NewDecoder(conn).Decode(&rep); err != nil {
		return nil, errors.New("failed to decode from conn:\n\t" + err.Error())
	}
	log.Println(config.RECEIVED, config.SERVER_HELLO)
	if err = conn.SetReadDeadline(time.Time{}); err != nil {
		return nil, errors.New("failed to reset read timeout:\n\t" + err.Error())
	}

	return &rep, nil
}

type reply struct {
	Hello     string            `json:"hello"`
	PublicKey ed25519.PublicKey `json:"public_key"`
	Options   map[string][]byte `json:"options"`
}

func NewReply(hello string, publicKey ed25519.PublicKey, options map[string][]byte) (*reply, error) {
	if hello != config.SERVER_HELLO {
		return nil, errors.New("failed to new request:\n\thello message expected to be SERVER_HELLO")
	}
	if len(publicKey) != config.X25519_KEY_SIZE {
		return nil, errors.New("failed to new request:\n\tpublicKey is not a valid X25519 public key")
	}
	return &reply{
		Hello:     hello,
		PublicKey: publicKey,
		Options:   options,
	}, nil
}

func (r *reply) SendReply(conn net.Conn) error {
	jrep, err := json.Marshal(r)
	if err != nil {
		return errors.New("failed to marshell reply:\n\t" + err.Error())
	}
	if err = conn.SetWriteDeadline(time.Now().Add(config.CONN_TIMEOUT)); err != nil {
		return errors.New("failed to set conn write timeout:\n\t" + err.Error())
	}
	if _, err = conn.Write(jrep); err != nil {
		return errors.New("failed to write to conn:\n\t" + err.Error())
	}
	log.Println(config.SENT, config.SERVER_HELLO)
	if err = conn.SetWriteDeadline(time.Time{}); err != nil {
		return errors.New("failed to reset write timeout:\n\t" + err.Error())
	}

	return nil
}
