package conn

import (
	"bufio"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"net"
	"simulation/config"
	"simulation/pki"

	"github.com/spf13/viper"
)

type edgeConn struct {
	protocol string
	addr     string
	port     int
	conn     net.Conn
	crypto   bool
	keyPair  *pki.KeyPair
	rx       []byte
	tx       []byte
	txHeader []byte
	sender   pki.Encryptor
	receiver pki.Decryptor
}

func NewEdgeConn(clinetCfg string) (*edgeConn, error) {
	var err error
	viper.SetConfigFile(clinetCfg)
	if err = viper.ReadInConfig(); err != nil {
		return nil, errors.New("failed to read config:\n\t" + err.Error())
	}

	ec := edgeConn{}
	ec.protocol = viper.GetString("client.protocol")
	ec.addr = viper.GetString("client.addr")
	ec.port = viper.GetInt("client.port")

	ec.conn, err = net.Dial(ec.protocol, fmt.Sprintf("%s:%v", ec.addr, ec.port))
	if err != nil {
		return nil, errors.New("failed to dail:\n\t" + err.Error())
	}

	ec.crypto = viper.IsSet("client.crypto")
	if ec.crypto {
		if ec.keyPair, err = pki.NewKeyPair(); err != nil {
			return nil, errors.New("failed to new keyPair:\n\t" + err.Error())
		}
	} else {
		ec.keyPair = nil
	}

	ec.txHeader = make([]byte, config.STREAM_HEADER_SIZE)
	if _, err := rand.Read(ec.txHeader); err != nil {
		return nil, errors.New("failed to make txHeader:\n\t" + err.Error())
	}

	return &ec, nil
}

func (c *edgeConn) Connect() error {
	options := map[string][]byte{
		config.TX_HEADER: c.txHeader,
	}
	req, err := NewRequest(config.CLIENT_HELLO, c.keyPair.Public(), options)
	if err != nil {
		return errors.New("failed to connect:\n\t" + err.Error())
	}
	rep, err := req.SendForReply(c.conn)
	if err != nil {
		return errors.New("failed to get reply:\n\t" + err.Error())
	}

	if c.rx, c.tx, err = c.keyPair.ClientSessionKeys(rep.PublicKey); err != nil {
		return errors.New("failed to compute rx tx :\n\t" + err.Error())
	}
	// fmt.Println("rx:", c.rx)
	// fmt.Println("tx:", c.tx)

	if c.sender, err = pki.NewEncryptor(c.tx, c.txHeader); err != nil {
		return errors.New("failed to new encryptor\n\t%s\n" + err.Error())
	}
	if c.receiver, err = pki.NewDecryptor(c.rx, rep.Options[config.TX_HEADER]); err != nil {
		return errors.New("failed to new decryptor\n\t%s\n" + err.Error())
	}
	return nil
}

func (c *edgeConn) Communicate() error {
	plaintextSend := "simulation"
	if err := c.write([]byte(plaintextSend)); err != nil {
		return errors.New("failed to write:\n\t" + err.Error())
	}

	if err := c.read(); err != nil {
		return errors.New("failed to read:\n\t" + err.Error())
	}

	return nil
}

func (c *edgeConn) read() error {
	reader := bufio.NewReader(c.conn)

	ciphertextReceive, err := reader.ReadString('\n')
	if err != nil {
		if err.Error() == "EOF" {
			return errors.New("Unexpected error:\n\t" + err.Error())
		}
		return errors.New("failed to read from conn:\n\t" + err.Error())
	}
	log.Println(config.RECEIVED, ciphertextReceive)
	ciphertextReceive = ciphertextReceive[:len(ciphertextReceive)-1]
	
	plaintextReceive, _, err := c.receiver.Pull([]byte(ciphertextReceive))
	if err != nil {
		return errors.New("failed to decrypt ciphertext:\n\t%s\n" + err.Error())
	}
	log.Println(config.PLAINTEXT, string(plaintextReceive))
	return nil
}

func (c *edgeConn) write(plaintextSend []byte) error {
	log.Println(config.PLAINTEXT, string(plaintextSend))

	ciphertextSend, err := c.sender.Push([]byte(plaintextSend), config.TAG_MESSAGE)
	if err != nil {
		return errors.New("failed to encrypt plaintext:\n\t" + err.Error())
	}
	ciphertextSend = append(ciphertextSend, '\n')
	if _, err = c.conn.Write(ciphertextSend); err != nil {
		return errors.New("failed to write to conn:\n\t" + err.Error())
	}

	log.Println(config.SENT, string(ciphertextSend))
	return nil
}

func (c *edgeConn) Close() error {
	return c.conn.Close()
}
