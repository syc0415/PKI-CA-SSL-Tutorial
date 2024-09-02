# Handshake

```go
    conn, err := context.Dial(serviceName)
```

```go
    // sdk-golang/ziti/ziti.go
    func (context *ContextImpl) Dial(serviceName string) (edge.Conn, error) {
        // ~~~
        return context.DialWithOptions(serviceName, defaultOptions)
    }
```

```go
    // sdk-golang/ziti/ziti.go
    func (context *ContextImpl) DialWithOptions(serviceName string, options *DialOptions) (edge.Conn, error) {
        // ~~~
        conn, err := context.dialSession(svc, session, edgeDialOptions)
        //~~~
    }
```

```go
    // sdk-golang/ziti/ziti.go
    func (context *ContextImpl) dialSession(service *rest_model.ServiceDetail, session *rest_model.SessionDetail, options *edge.DialOptions) (edge.Conn, error) {
        edgeConnFactory, err := context.getEdgeRouterConn(session, options)
        if err != nil {
            return nil, err
        }
        return edgeConnFactory.Connect(service, session, options)
    }
```

```go
    // sdk-golang/ziti/edge/network/conn.go
    func (conn *edgeConn) Connect(session *rest_model.SessionDetail, options *edge.DialOptions) (edge.Conn, error) {
        // ~~~
        var pub []byte
        if conn.crypto {
            pub = conn.keyPair.Public()
        }
        connectRequest := edge.NewConnectMsg(conn.Id(), *session.Token, pub, options)
        connectRequest.Headers[edge.ConnectionMarkerHeader] = []byte(conn.marker)
        conn.TraceMsg("connect", connectRequest)
        replyMsg, err := connectRequest.WithTimeout(options.ConnectTimeout).SendForReply(conn.Channel)
        // ~~~
        if conn.crypto {
            method, _ := replyMsg.GetByteHeader(edge.CryptoMethodHeader)
            hostPubKey := replyMsg.Headers[edge.PublicKeyHeader]
            if hostPubKey != nil {
                logger.Debug("setting up end-to-end encryption")
                if err = conn.establishClientCrypto(conn.keyPair, hostPubKey, edge.CryptoMethod(method)); err != nil {
                    logger.WithError(err).Error("crypto failure")
                    _ = conn.Close()
                    return nil, err
                }
                logger.Debug("client tx encryption setup done")
            } else {
                logger.Warn("connection is not end-to-end-encrypted")
            }
        }
        // ~~~
    }
```

```go
    // sdk-golang/ziti/edge/network/conn.go
    func (conn *edgeConn) establishClientCrypto(keypair *kx.KeyPair, peerKey []byte, method edge.CryptoMethod) error {
        var err error
        var rx, tx []byte

        if method != edge.CryptoMethodLibsodium {
            return unsupportedCrypto
        }

        if rx, tx, err = keypair.ClientSessionKeys(peerKey); err != nil {
            return errors.Wrap(err, "failed key exchange")
        }

        var txHeader []byte
        if conn.sender, txHeader, err = secretstream.NewEncryptor(tx); err != nil {
            return errors.Wrap(err, "failed to establish crypto stream")
        }

        conn.rxKey = rx

        if _, err = conn.MsgChannel.Write(txHeader); err != nil {
            return errors.Wrap(err, "failed to write crypto header")
        }

        pfxlog.Logger().
            WithField("connId", conn.Id()).
            WithField("marker", conn.marker).
            Debug("crypto established")
        return nil
    }
```

```go
    // secretstream/kx/kx.go
    func (pair *KeyPair) ClientSessionKeys(server_pk []byte) (rx []byte, tx []byte, err error) {
        q, err := curve25519.X25519(pair.sk[:], server_pk)
        if err != nil {
            return nil, nil, err
        }

        h, err := blake2b.New(2*SessionKeyBytes, nil)
        if err != nil {
            return nil, nil, err
        }

        for _, b := range [][]byte{q, pair.Public(), server_pk} {
            if _, err = h.Write(b); err != nil {
                return nil, nil, err
            }
        }

        keys := h.Sum(nil)

        return keys[:SessionKeyBytes], keys[SecretKeyBytes:], nil
    }
    func (pair *KeyPair) ServerSessionKeys(client_pk []byte) (rx []byte, tx []byte, err error) {
        q, err := curve25519.X25519(pair.sk[:], client_pk)
        if err != nil {
            return nil, nil, err
        }

        h, err := blake2b.New(2*SessionKeyBytes, nil)
        if err != nil {
            return nil, nil, err
        }

        for _, b := range [][]byte{q, client_pk, pair.Public()} {
            if _, err = h.Write(b); err != nil {
                return nil, nil, err
            }
        }

        keys := h.Sum(nil)

        return keys[SessionKeyBytes:], keys[:SecretKeyBytes], nil
    }
```
