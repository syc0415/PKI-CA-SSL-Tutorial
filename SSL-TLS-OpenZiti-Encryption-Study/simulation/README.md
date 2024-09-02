# Ziti's Model Simulation

## Env

|-|Version|
|-|-|
|Ubuntu|22.04.4 LTS|
|Golang|go1.22.5 linux/amd64|

## Sequence Diagram

```mermaid
sequenceDiagram
    participant c as Client
    participant s as Server

    c -->> c: 1. KeyPairC, txHeaderC
    c ->> s: 2. clientHello
    s -->> s: 3. KeyPairS, rxS, txS, txHeaderS, senderS, receiverS
    s ->> c: 4. serverHello
    c -->> c: 5. rxC, txC, senderC, receiverC
    Note over c, s: Crypto Communication
```

## Crypto Model - Base Curve25519

### Prep: Exchange Public Key and txHeader

```mermaid
graph LR
    Server[Server] -->|pkS, txHeaderS| Client[Client]
    Client -->|pkC, txHeaderC| Server
```

### Crypto

```mermaid
graph TB
    subgraph Computation
        direction LR

        subgraph ServerComputation
            direction TB
            
            SA[SessionKey - skS] -->|+pkC| SC[SharePoint - q]
            SB[PublicKey - pkS] --> SD["Key = hash(q + pkC + pkS)"]
            SC --> SD
            SD -->|"Key[32:64]"| SE[rxS]
            SD -->|"Key[0:32]"| SF[txS]
        end

        subgraph ClientComputation
            direction TB
            
            CA[SessionKey - skC] -->|+pkS| CC[SharePoint - q]
            CB[PublicKey - pkC] --> CD["Key = hash(q + pkC + pkS)"]
            CC --> CD
            CD -->|"Key[0:32]"| CE[rxC]
            CD -->|"Key[32:64]"| CF[txC]
        end
    end

    subgraph Communication
        direction LR

        subgraph ServerCommunication
            direction TB
            A[DecryptorS]
            B[EncryptorS]
        end

        subgraph ClientCommunication
            direction TB
            C[DecryptorC]
            D[EncryptorC]
        end

        B --> C
        D --> A
    end



    SE -->|+txHeaderC| A
    SF -->|+txHeaderS| B
    CE -->|+txHeaderS| C
    CF -->|+txHeaderC| D
```

+ Keypoint:
  + The two SharePoints, q, in both ServerComputation and ClientComputation are the same.
  + Since q is the same, it fllows that the Keys in both side, which are derived from hash q + pkC + pkS, are also the same.
  + And also, txS(Key[0:32]) is same as rxC(Key[0:32]). They will plus txHeaderS to make cryptor. So, DecryptorC can decrypt the cipher from EncryptorS.
  + By the same reasoning, DecryptorS can decrypt the cipher from EncryptorC.

## Demo

### Server

Path: SSL-TLS-OpenZiti-Encryption-Study/simulation/server

Run:

```bash
    go test
```

### Client

Path: SSL-TLS-OpenZiti-Encryption-Study/simulation/client

Run:

```bash
    go test
```

### ScreenShots

![server](../static/simulationImg/server.png)
![client](../static/simulationImg/client.png)

+ Description:
  + The client will encrypt the string "simulation" and send its ciphertext to the server.
  + The server will receive the ciphertext, decrypt it, reverse the plaintext, and then encrypt the reversed plaintext before sending it back to the client.
  + The client will receive the ciphertext, decrypt it, and obtain the correct plaintext "noitalumis".

## Reference

[Openziti: sdk-golang](https://github.com/openziti/sdk-golang)

[Openziti: secretstream](https://github.com/openziti/secretstream)

[Golang Package](https://pkg.go.dev/)
