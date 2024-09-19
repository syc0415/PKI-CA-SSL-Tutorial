# Encrypt

```go
    conn.Write([]byte(""))
```

```go
    // sdk-golang/ziti/edge/network/conn.go
    func (conn *edgeConn) Write(data []byte) (int, error) {
        if conn.sentFIN.Load() {
            return 0, errors.New("calling Write() after CloseWrite()")
        }

        if conn.sender != nil {
            cipherData, err := conn.sender.Push(data, secretstream.TagMessage)
            if err != nil {
                return 0, err
            }

            _, err = conn.MsgChannel.Write(cipherData)
            return len(data), err
        } else {
            return conn.MsgChannel.Write(data)
        }
    }
    func (conn *edgeConn) Read(p []byte) (int, error) {
        log := pfxlog.Logger().WithField("connId", conn.Id()).WithField("marker", conn.marker)
        if conn.closed.Load() {
            return 0, io.EOF
        }

        log.Tracef("read buffer = %d bytes", len(p))
        if len(conn.leftover) > 0 {
            log.Tracef("found %d leftover bytes", len(conn.leftover))
            n := copy(p, conn.leftover)
            conn.leftover = conn.leftover[n:]
            return n, nil
        }

        for {
            if conn.readFIN.Load() {
                return 0, io.EOF
            }

            msg, err := conn.readQ.GetNext()
            if err == ErrClosed {
                log.Debug("sequencer closed, closing connection")
                conn.closed.Store(true)
                return 0, io.EOF
            } else if err != nil {
                log.Debugf("unexpected sequencer err (%v)", err)
                return 0, err
            }

            flags, _ := msg.GetUint32Header(edge.FlagsHeader)
            if flags&edge.FIN != 0 {
                conn.readFIN.Store(true)
            }

            switch msg.ContentType {

            case edge.ContentTypeStateClosed:
                log.Debug("received ConnState_CLOSED message, closing connection")
                conn.close(true)
                continue

            case edge.ContentTypeData:
                d := msg.Body
                log.Tracef("got buffer from sequencer %d bytes", len(d))
                if len(d) == 0 && conn.readFIN.Load() {
                    return 0, io.EOF
                }

                if conn.rxKey != nil {
                    if len(d) != secretstream.StreamHeaderBytes {
                        return 0, errors.Errorf("failed to receive crypto header bytes: read[%d]", len(d))
                    }
                    conn.receiver, err = secretstream.NewDecryptor(conn.rxKey, d)
                    if err != nil {
                        return 0, errors.Wrap(err, "failed to init decryptor")
                    }
                    conn.rxKey = nil
                    continue
                }

                if conn.receiver != nil {
                    d, _, err = conn.receiver.Pull(d)
                    if err != nil {
                        log.WithFields(edge.GetLoggerFields(msg)).Errorf("crypto failed on msg of size=%v, headers=%+v err=(%v)", len(msg.Body), msg.Headers, err)
                        return 0, err
                    }
                }
                n := copy(p, d)
                conn.leftover = d[n:]

                log.Tracef("saving %d bytes for leftover", len(conn.leftover))
                log.Debugf("reading %v bytes", n)
                return n, nil

            default:
                log.WithField("type", msg.ContentType).Error("unexpected message")
            }
        }
    }
```

```go
    // secretstream/stream.go
    func NewEncryptor(key []byte) (Encryptor, []byte, error) {
        if len(key) != StreamKeyBytes {
            return nil, nil, invalidKey
        }

        header := make([]byte, StreamHeaderBytes)
        _, err := rand.Read(header)
        if err != nil {
            return nil, nil, err
        }

        stream := &encryptor{}

        k, err := chacha20.HChaCha20(key[:], header[:16])
        if err != nil {
            return nil, nil, err
        }
        copy(stream.k[:], k)
        stream.reset()

        for i := range stream.pad {
            stream.pad[i] = 0
        }

        for i, b := range header[crypto_core_hchacha20_INPUTBYTES:] {
            stream.nonce[i+crypto_secretstream_xchacha20poly1305_COUNTERBYTES] = b
        }

        return stream, header, nil
    }
    func (s *encryptor) Push(plain []byte, tag byte) ([]byte, error) {
        var err error
        var poly *poly1305.MAC
        var block [64]byte
        var slen [8]byte

        mlen := len(plain)
        out := make([]byte, mlen+StreamABytes)

        chacha, err := chacha20.NewUnauthenticatedCipher(s.k[:], s.nonce[:])
        if err != nil {
            return nil, err
        }
        
        chacha.XORKeyStream(block[:], block[:])

        var poly_init [32]byte
        copy(poly_init[:], block[:])
        poly = poly1305.New(&poly_init)

        memzero(block[:])
        block[0] = tag

        chacha.XORKeyStream(block[:], block[:])
        _, _ = poly.Write(block[:])
        out[0] = block[0]

        c := out[1:]
        chacha.XORKeyStream(c, plain)
        _, _ = poly.Write(c[:mlen])
        padlen := (0x10 - len(block) + mlen) & 0xf
        _, _ = poly.Write(pad0[:padlen])

        binary.LittleEndian.PutUint64(slen[:], uint64(0))
        _, _ = poly.Write(slen[:])

        binary.LittleEndian.PutUint64(slen[:], uint64(len(block)+mlen))
        _, _ = poly.Write(slen[:])

        mac := c[mlen:]
        copy(mac, poly.Sum(nil))

        xor_buf(s.nonce[crypto_secretstream_xchacha20poly1305_COUNTERBYTES:], mac)
        buf_inc(s.nonce[:crypto_secretstream_xchacha20poly1305_COUNTERBYTES])

        return out, nil
    }
    func NewDecryptor(key, header []byte) (Decryptor, error) {
        stream := &decryptor{}
        k, err := chacha20.HChaCha20(key, header[:16])
        if err != nil {
            fmt.Printf("error: %v", err)
            return nil, err
        }
        copy(stream.k[:], k)

        stream.reset()

        copy(stream.nonce[crypto_secretstream_xchacha20poly1305_COUNTERBYTES:],
            header[crypto_core_hchacha20_INPUTBYTES:])

        copy(stream.pad[:], pad0[:])

        return stream, nil
    }

    func (s *decryptor) Pull(in []byte) ([]byte, byte, error) {
        inlen := len(in)

        var block [64]byte

        var slen [8]byte

        if inlen < StreamABytes {
            return nil, 0, invalidInput
        }

        mlen := inlen - StreamABytes

        chacha, err := chacha20.NewUnauthenticatedCipher(s.k[:], s.nonce[:])
        if err != nil {
            return nil, 0, err
        }

        chacha.XORKeyStream(block[:], block[:])

        var poly_init [32]byte
        copy(poly_init[:], block[:])
        poly := poly1305.New(&poly_init)

        memzero(block[:])
        block[0] = in[0]

        chacha.XORKeyStream(block[:], block[:])
        tag := block[0]
        block[0] = in[0]
        if _, err = poly.Write(block[:]); err != nil {
            return nil, 0, err
        }

        c := in[1:]
        if _, err = poly.Write(c[:mlen]); err != nil {
            return nil, 0, err
        }

        padlen := (0x10 - len(block) + mlen) & 0xf
        if _, err = poly.Write(pad0[:padlen]); err != nil {
            return nil, 0, err
        }

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
            return nil, 0, cryptoFailure
        }
        m := make([]byte, mlen)
        chacha.XORKeyStream(m, c[:mlen])

        xor_buf(s.nonce[crypto_secretstream_xchacha20poly1305_COUNTERBYTES:], mac)
        buf_inc(s.nonce[:crypto_secretstream_xchacha20poly1305_COUNTERBYTES])

        return m, tag, nil
    }

```

```go
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
```
