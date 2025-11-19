# Signature and Verification Flows in Rubix Platform

Complete guide to signature modes, verification modes, and token transfer flows with detailed emphasis on WalletDID mode.

---

## Table of Contents

1. [Signature Modes](#signature-modes)
2. [Verification Modes](#verification-modes)
3. [Complete Token Transfer Flow](#complete-token-transfer-flow)
4. [WalletDID Mode Flow](#walletdid-mode-detailed-flow)
5. [Signature Verification Points](#signature-verification-points)
6. [API Flow Diagrams](#api-flow-diagrams)

---

## Signature Modes

There are **2 primary signature modes** in Rubix, determined by the DID type:

### 1. NLSS Signature Mode (NlssVersion = 1)

**Used by:**
- BasicDIDMode
- StandardDIDMode
- WalletDIDMode
- ChildDIDMode
- QuorumDIDMode

**Signing Function:** `dc.Sign(hash string) ([]byte, []byte, error)`

**Returns:**
- `nlssShare` (32 bytes) - 256 bits extracted from pvtShare.png
- `privateSign` (bytes) - ECDSA-P256 signature over SHA3-256(nlssShare)

**Code Location:** `did/basic.go:105-144`, `did/wallet.go:66-71`

**Implementation:**
```go
func (d *DIDBasic) Sign(hash string) ([]byte, []byte, error) {
    // Step 1: Load pvtShare.png
    byteImg := util.GetPNGImagePixels(d.dir + PvtShareFileName)
    ps := util.ByteArraytoIntArray(byteImg)

    // Step 2: Derive 256 positions using hash chaining
    randPosObject := util.RandomPositions("signer", hash, 32, ps)

    // Step 3: Extract 256 bits from pvtShare at derived positions
    finalPos := randPosObject.PosForSign
    pvtPos := util.GetPrivatePositions(finalPos, ps)
    pvtPosStr := util.IntArraytoStr(pvtPos)

    // Step 4: Load and decrypt private key
    privKey := ioutil.ReadFile(d.dir + PvtKeyFileName)
    pwd := d.getPassword()  // May trigger password request
    PrivateKey := crypto.DecodeKeyPair(pwd, privKey, nil)

    // Step 5: Sign the extracted bits with ECDSA
    hashPvtSign := util.HexToStr(util.CalculateHash([]byte(pvtPosStr), "SHA3-256"))
    pvtKeySign := crypto.Sign(PrivateKey, []byte(hashPvtSign))

    // Step 6: Convert to bytes
    bs := util.BitstreamToBytes(pvtPosStr)

    return bs, pvtKeySign, nil  // (32 bytes, ECDSA sig)
}
```

**WalletDID Implementation:**
```go
func (d *DIDWallet) Sign(hash string) ([]byte, []byte, error) {
    // Delegates to external wallet via channel
    bs, pvtKeySign := d.getSignature([]byte(hash), false)
    return bs, pvtKeySign, nil
}
```

---

### 2. BIP Signature Mode (BIPVersion = 0)

**Used by:**
- LiteDIDMode
- QuorumLiteDIDMode

**Signing Function:** `dc.Sign(hash string) ([]byte, []byte, error)`

**Returns:**
- `[]byte{}` - Empty (no NLSS component)
- `privateSign` (bytes) - secp256k1 signature over hash

**Code Location:** `did/lite.go:108-112`

**Implementation:**
```go
func (d *DIDLite) Sign(hash string) ([]byte, []byte, error) {
    // Call PvtSign which uses secp256k1
    pvtKeySign := d.PvtSign([]byte(hash))
    bs := []byte{}  // Empty NLSS component

    return bs, pvtKeySign, nil  // (empty, secp256k1 sig)
}

func (d *DIDLite) PvtSign(hash []byte) ([]byte, error) {
    // Try local key first
    privKey := os.ReadFile(d.dir + PvtKeyFileName)
    if err != nil {
        // Fall back to wallet
        return d.getSignature(hash)
    }

    // Decrypt BIP key
    pwd := d.getPassword()
    Privatekey := crypto.DecodeBIPKeyPair(pwd, privKey, nil)

    // Sign with secp256k1
    privkeyback := secp256k1.PrivKeyFromBytes(Privatekey)
    privKeySer := privkeyback.ToECDSA()
    pvtKeySign := crypto.BIPSign(privKeySer, hash)

    return pvtKeySign, nil
}
```

---

## Verification Modes

There are **2 verification modes** corresponding to the signature modes:

### 1. NLSS Verification Mode

**Function:** `dc.NlssVerify(hash string, pvtShareSig []byte, pvtKeySIg []byte) (bool, error)`

**Code Location:** `did/basic.go:148-197`

**Steps:**

```go
func (d *DIDBasic) NlssVerify(hash string, pvtShareSig []byte, pvtKeySIg []byte) (bool, error) {
    // Step 1: Load public files
    didImg := util.GetPNGImagePixels(d.dir + DIDImgFileName)
    pubImg := util.GetPNGImagePixels(d.dir + PubShareFileName)

    // Step 2: Convert signature to bit array
    pSig := util.BytesToBitstream(pvtShareSig)
    ps := util.StringToIntArray(pSig)

    // Step 3: Derive positions using signature bits (hash chaining)
    pubPos := util.RandomPositions("verifier", hash, 32, ps)
    //                               ^^^^^^^^        ^^
    //                            Role=verifier   Signature bits

    // Step 4: Extract bits from pubShare at derived positions
    pubPosInt := util.GetPrivatePositions(pubPos.PosForSign, pubBin)
    pubStr := util.IntArraytoStr(pubPosInt)

    // Step 5: Extract bits from DID at same positions
    orgPos := make([]int, len(pubPos.OriginalPos))
    for i := range pubPos.OriginalPos {
        orgPos[i] = pubPos.OriginalPos[i] / 8
    }
    didPosInt := util.GetPrivatePositions(orgPos, didBin)
    didStr := util.IntArraytoStr(didPosInt)

    // Step 6: NLSS reconstruction (dot product)
    cb := nlss.Combine2Shares(nlss.ConvertBitString(pSig), nlss.ConvertBitString(pubStr))
    db := nlss.ConvertBitString(didStr)

    // Step 7: Check NLSS verification
    if !bytes.Equal(cb, db) {
        return false, fmt.Errorf("failed to verify")
    }

    // Step 8: Load public key and verify ECDSA signature
    pubKey := ioutil.ReadFile(d.dir + PubKeyFileName)
    _, pubKeyByte := crypto.DecodeKeyPair("", nil, pubKey)
    hashPvtSign := util.HexToStr(util.CalculateHash([]byte(pSig), "SHA3-256"))
    if !crypto.Verify(pubKeyByte, []byte(hashPvtSign), pvtKeySIg) {
        return false, fmt.Errorf("failed to verify nlss private key singature")
    }

    return true, nil
}
```

**Verification Points:**
1. ✅ NLSS dot product: `pvtShareSig · pubShare = DID` (quantum-resistant layer)
2. ✅ ECDSA signature: `verify(pubKey, SHA3-256(pvtShareSig), pvtKeySig)` (PKI layer)

---

### 2. BIP Verification Mode

**Function:** `dc.PvtVerify(hash []byte, sign []byte) (bool, error)`

**Code Location:** `did/lite.go:210-226`

**Steps:**

```go
func (d *DIDLite) PvtVerify(hash []byte, sign []byte) (bool, error) {
    // Step 1: Load public key
    pubKey := ioutil.ReadFile(d.dir + PubKeyFileName)

    // Step 2: Decode BIP public key
    _, pubKeyByte := crypto.DecodeBIPKeyPair("", nil, pubKey)

    // Step 3: Verify secp256k1 signature
    pubkeyback := secp256k1.ParsePubKey(pubKeyByte)
    pubKeySer := pubkeyback.ToECDSA()
    if !crypto.BIPVerify(pubKeySer, hash, sign) {
        return false, fmt.Errorf("failed to verify private key singature")
    }

    return true, nil
}
```

**Verification Points:**
1. ✅ secp256k1 signature: `verify(pubKey, hash, signature)`

---

## Complete Token Transfer Flow

### Overview

```
┌─────────────┐
│   Client    │
│  (Wallet)   │
└──────┬──────┘
       │
       │ POST /api/initiate-rbt-transfer
       │ {sender, receiver, tokenCount, type, comment}
       │
       ▼
┌─────────────────────────────────────────────┐
│          Node API Server                     │
│  (server/tokens.go:APIInitiateRBTTransfer)  │
└──────┬──────────────────────────────────────┘
       │
       │ 1. Validate request
       │ 2. Create web request channel
       │ 3. Launch goroutine: InitiateRBTTransfer()
       │ 4. Return immediate response with reqID
       │
       ▼
┌─────────────────────────────────────────────┐
│    didResponse() - Waits for Completion     │
│  Returns: {id, status, message, result}     │
└──────┬──────────────────────────────────────┘
       │
       │ Client receives response with:
       │ - id: Request ID (for signature response)
       │ - status: Processing started
       │ - message: "Password needed" or similar
       │
       ▼
```

### Detailed Flow

#### Phase 1: API Request Initiation

**File:** `server/tokens.go:74-115`

```go
func (s *Server) APIInitiateRBTTransfer(req *ensweb.Request) *ensweb.Result {
    var rbtReq model.RBTTransferRequest
    s.ParseJSON(req, &rbtReq)

    // Validate sender and receiver DIDs
    _, senderDID := util.ParseAddress(rbtReq.Sender)
    _, receiverDID := util.ParseAddress(rbtReq.Receiver)

    // Validate amounts and types
    if rbtReq.TokenCount < 0.001 {
        return s.BasicResponse(req, false, "Invalid RBT amount")
    }

    // Check DID access
    if !s.validateDIDAccess(req, rbtReq.Sender) {
        return s.BasicResponse(req, false, "DID does not have access")
    }

    // Create web request channel for async communication
    s.c.AddWebReq(req)

    // Launch transfer in goroutine
    go s.c.InitiateRBTTransfer(req.ID, &rbtReq)

    // Return immediately with request ID
    return s.didResponse(req, req.ID)
}
```

**Response 1 (Immediate):**
```json
{
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "status": false,
    "message": "Processing...",
    "result": null
}
```

---

#### Phase 2: Token Gathering and Validation

**File:** `core/transfer.go:183-236`

```go
func (c *Core) initiateRBTTransfer(reqID string, req *model.RBTTransferRequest) {
    // Step 1: Setup DID (initializes appropriate DID mode)
    dc, err := c.SetupDID(reqID, req.Sender)
    // This calls: did.InitDIDBasic() or did.InitDIDWallet() etc.

    // Step 2: Gather tokens for transaction
    tokensForTxn, err := gatherTokensForTransaction(c, req, dc, isSelfTransfer)
    // - Checks account balance
    // - Locks required tokens
    // - Validates sufficient balance

    // Step 3: Get receiver peer ID
    rpeerid := c.w.GetPeerID(receiverdid)

    // Step 4: Build token info list
    for each token {
        - Get latest block
        - Get block ID
        - Create TokenInfo
    }

    // Step 5: Create smart contract
    sc := contract.CreateNewContract(contractType)

    // Step 6: REQUEST SIGNATURE (this is where password/wallet is needed)
    err = sc.UpdateSignature(dc)
    // This calls dc.Sign(hash) internally

    // Continue...
}
```

---

#### Phase 3: Signature Request (Critical Point)

**File:** `contract/contract.go:394-449`

```go
func (c *Contract) UpdateSignature(dc did.DIDCrypto) error {
    did := dc.GetDID()

    // Step 1: Get hash of smart contract
    hash, err := c.GetHash()

    // Step 2: SIGN THE CONTRACT
    // This is where the signature request happens!
    ssig, psig, err := dc.Sign(hash)
    // For WalletDID: sends request to wallet
    // For BasicDID: requests password

    // Step 3: Store signatures in contract
    c.sm[SCShareSignatureKey][did] = util.HexToStr(ssig)
    c.sm[SCKeySignatureKey][did] = util.HexToStr(psig)

    return nil
}
```

**At this point:**
- If **WalletDID**: Request sent to external wallet, waiting for response
- If **BasicDID**: Password request sent to client, waiting for response
- If **StandardDID**: Request sent to remote signer, waiting for response

---

#### Phase 4: Client Provides Signature/Password

**Scenario A: BasicDIDMode (Password Flow)**

**Step 1:** Node requests password via channel:
```go
// In did/basic.go:getPassword()
sr := &SignResponse{
    Status:  true,
    Message: "Password needed",
    Result: SignReqData{
        ID:   reqID,
        Mode: BasicDIDMode,
    },
}
dc.OutChan <- sr  // Sent to client
```

**Step 2:** Client receives response:
```json
{
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "status": true,
    "message": "Password needed",
    "result": {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "mode": 0
    }
}
```

**Step 3:** Client sends password:
```
POST /api/signature-response
{
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "mode": 0,
    "password": "user_password_here"
}
```

**Step 4:** Server processes password response:

**File:** `server/tokens.go:242-255`

```go
func (s *Server) APISignatureResponse(req *ensweb.Request) *ensweb.Result {
    var resp did.SignRespData
    s.ParseJSON(req, &resp)

    // Get the waiting channel
    dc := s.c.GetWebReq(resp.ID)
    if dc == nil {
        return s.BasicResponse(req, false, "Invalid request ID", nil)
    }

    // Send password back to signing process
    dc.InChan <- resp  // Unblocks getPassword()

    // Return status
    return s.didResponse(req, resp.ID)
}
```

**Step 5:** Signing completes with password:
```go
// Back in did/basic.go:getPassword()
srd := <-d.ch.InChan  // Receives password
return srd.Password, nil

// Then in Sign():
PrivateKey := crypto.DecodeKeyPair(password, privKey, nil)
pvtKeySign := crypto.Sign(PrivateKey, hash)
// Signature complete!
```

---

**Scenario B: WalletDIDMode (Wallet Flow)**

**Step 1:** Node requests signature from wallet:
```go
// In did/wallet.go:getSignature()
sr := &SignResponse{
    Status:  true,
    Message: "Signature needed",
    Result: SignReqData{
        ID:          reqID,
        Mode:        WalletDIDMode,
        Hash:        hash,
        OnlyPrivKey: false,  // Need both NLSS and ECDSA
    },
}
d.ch.OutChan <- sr  // Sent to wallet interface
```

**Step 2:** Client receives signature request:
```json
{
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "status": true,
    "message": "Signature needed",
    "result": {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "mode": 2,
        "hash": "a1b2c3d4...",
        "only_priv_key": false
    }
}
```

**Step 3:** Wallet application performs signing:

```javascript
// Wallet app (e.g., browser extension, hardware wallet)
async function signTransaction(request) {
    // 1. Load pvtShare from secure storage
    const pvtShare = await loadFromSecureElement('pvtShare.png');

    // 2. Derive positions using hash chaining
    const positions = randomPositions('signer', request.hash, 32, pvtShare);

    // 3. Extract 256 bits from pvtShare
    const nlssShare = extractBits(pvtShare, positions);  // 32 bytes

    // 4. Load private key from secure storage
    const privateKey = await loadFromSecureElement('pvtKey.pem');

    // 5. Sign with ECDSA
    const hash = sha3_256(nlssShare);
    const privateSign = ecdsaSign(privateKey, hash);

    // 6. Return signature
    return {
        pixels: nlssShare,
        signature: privateSign
    };
}
```

**Step 4:** Wallet sends signature back:
```
POST /api/signature-response
{
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "mode": 2,
    "password": "",
    "signature": {
        "pixels": "base64_encoded_32_bytes",
        "signature": "base64_encoded_ecdsa_sig"
    }
}
```

**Step 5:** Server receives and processes:
```go
func (s *Server) APISignatureResponse(req *ensweb.Request) {
    var resp did.SignRespData
    s.ParseJSON(req, &resp)

    dc := s.c.GetWebReq(resp.ID)
    dc.InChan <- resp  // Unblocks getSignature()

    return s.didResponse(req, resp.ID)
}
```

**Step 6:** Signing completes with wallet signature:
```go
// Back in did/wallet.go:getSignature()
srd := <-d.ch.InChan  // Receives signature from wallet
return srd.Signature.Pixels, srd.Signature.Signature, nil
```

---

#### Phase 5: Consensus and Quorum Signing

**File:** `core/transfer.go:410-421`

```go
// After sender signs the contract
cr := getConsensusRequest(req.Type, c.peerID, rpeerid, sc.GetBlock(), txEpoch, isSelfTransfer)

go func() {
    // Initiate consensus with quorums
    td, _, pds, consError := c.initiateConsensus(cr, sc, dc)
    if consError != nil {
        resp.Message = "Consensus failed " + consError.Error()
        return
    }

    // Consensus succeeded, add to transaction history
    c.w.AddTransactionHistory(td)

    resp.Status = true
    resp.Message = "Transfer finished successfully"
}()
```

**During Consensus:**
- Contact quorum nodes
- Each quorum signs the token chain block
- Collect quorum signatures
- Verify all signatures
- Commit transaction

---

#### Phase 6: Final Response

**After consensus completes:**

```go
resp.Status = true
resp.Message = "Transfer finished successfully in 2.5s with trnxid abc123"
dc.OutChan <- resp
```

**Client receives final response:**
```json
{
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "status": true,
    "message": "Transfer finished successfully in 2.5s with trnxid abc123",
    "result": {
        "transaction_id": "abc123",
        "block_id": "block-xyz",
        "duration": 2500
    }
}
```

---

## WalletDID Mode Detailed Flow

### Complete Sequence Diagram

```
┌────────┐     ┌──────┐     ┌──────────┐     ┌────────┐
│ Client │     │ Node │     │ WalletDID│     │ Wallet │
│  App   │     │  API │     │  Module  │     │  App   │
└───┬────┘     └───┬──┘     └────┬─────┘     └───┬────┘
    │              │              │               │
    │ POST /api/   │              │               │
    │ initiate-rbt │              │               │
    │─────────────>│              │               │
    │              │              │               │
    │              │ SetupDID()   │               │
    │              │─────────────>│               │
    │              │              │               │
    │              │ InitDIDWallet│               │
    │              │<─────────────│               │
    │              │              │               │
    │<─────────────│              │               │
    │ Response 1:  │              │               │
    │ {id, status} │              │               │
    │              │              │               │
    │              │UpdateSignature()             │
    │              │─────────────>│               │
    │              │              │               │
    │              │              │ getSignature()│
    │              │              │──────────────>│
    │              │              │               │
    │              │              │ OutChan       │
    │              │              │<──────────────│
    │              │              │               │
    │              │ OutChan      │               │
    │              │<─────────────│               │
    │              │              │               │
    │<─────────────│              │               │
    │ Response 2:  │              │               │
    │ "Signature   │              │               │
    │  needed"     │              │               │
    │              │              │               │
    │ Display      │              │               │
    │ "Sign on     │              │               │
    │  Wallet"     │              │               │
    │              │              │               │
    │──────────────┼──────────────┼──────────────>│
    │              │              │               │
    │              │              │               │ User
    │              │              │               │ Confirms
    │              │              │               │
    │              │              │               │ Sign with
    │              │              │               │ pvtShare
    │              │              │               │ + pvtKey
    │              │              │               │
    │ POST /api/   │              │               │
    │ signature-   │              │               │
    │ response     │              │               │
    │─────────────>│              │               │
    │              │              │               │
    │              │ InChan       │               │
    │              │─────────────>│               │
    │              │              │               │
    │              │              │ InChan        │
    │              │              │──────────────>│
    │              │              │               │
    │              │              │ Return sig    │
    │              │              │<──────────────│
    │              │              │               │
    │              │ Signature    │               │
    │              │<─────────────│               │
    │              │              │               │
    │              │ Initiate     │               │
    │              │ Consensus    │               │
    │              │              │               │
    │              │ Contact      │               │
    │              │ Quorums      │               │
    │              │              │               │
    │              │ Verify       │               │
    │              │ Signatures   │               │
    │              │              │               │
    │              │ Commit       │               │
    │              │ Transaction  │               │
    │              │              │               │
    │              │ OutChan      │               │
    │              │<─────────────│               │
    │              │              │               │
    │<─────────────│              │               │
    │ Response 3:  │              │               │
    │ "Transfer    │              │               │
    │  successful" │              │               │
    │              │              │               │
    ▼              ▼              ▼               ▼
```

---

## Signature Verification Points

During a token transfer, signatures are verified at **multiple critical checkpoints**:

### Checkpoint 1: Sender Signature Verification

**Location:** `core/token_chain_validation.go:563-576`

**When:** During token chain block validation

**Code:**
```go
// Sender signature verification
if senderDIDType == did.LiteDIDMode {
    response.Status, err = didCrypto.PvtVerify(
        []byte(senderSign.Hash),
        util.StrToHex(senderSign.PrivateSign)
    )
} else {
    response.Status, err = didCrypto.NlssVerify(
        senderSign.Hash,
        util.StrToHex(senderSign.NLSSShare),
        util.StrToHex(senderSign.PrivateSign)
    )
}
if err != nil {
    c.log.Error("failed to verify sender:", sender)
    response.Message = "invalid sender"
    return response, err
}
```

**What's Verified:**
- ✅ Sender's NLSS signature (if NLSS mode)
- ✅ Sender's ECDSA signature
- ✅ Hash of token chain block matches

**Purpose:** Ensures the sender actually authorized this transfer

---

### Checkpoint 2: Quorum Signatures Verification

**Location:** `core/token_chain_validation.go:641-656`

**When:** After all quorums have signed

**Code:**
```go
for each quorum {
    qrmDIDCrypto, err := c.SetupForienDID(qrm.DID, selfDID)

    var verificationStatus bool
    if qrm.SignType == "0" {  // BIP mode
        verificationStatus, err = qrmDIDCrypto.PvtVerify(
            []byte(signedData),
            util.StrToHex(qrm.PrivSignature)
        )
    } else {  // NLSS mode
        verificationStatus, err = qrmDIDCrypto.NlssVerify(
            signedData,
            util.StrToHex(qrm.Signature),
            util.StrToHex(qrm.PrivSignature)
        )
    }

    response.Status = response.Status && verificationStatus
}
```

**What's Verified:**
- ✅ Each quorum's signature on the token chain block
- ✅ Quorum DID is valid
- ✅ All quorums agree on the same block hash

**Purpose:** Ensures consensus agreement on the transaction

---

### Checkpoint 3: Previous Block Signatures

**Location:** During token chain traversal

**When:** Validating token ownership history

**Code:**
```go
// Validate each block in the token chain
for each block in tokenChain {
    // Get signers from block
    signers, _ := block.GetSigner()

    for each signer {
        // Verify signer's signature on this block
        didCrypto := c.SetupForienDID(signer.DID, selfDID)

        verified, err := didCrypto.NlssVerify(
            block.Hash,
            signer.NLSSShare,
            signer.PrivateSign
        )

        if !verified {
            return fmt.Errorf("invalid signature in block %s", block.ID)
        }
    }
}
```

**What's Verified:**
- ✅ Historical signatures in token chain
- ✅ Chain of custody is valid
- ✅ No tampering in previous blocks

**Purpose:** Ensures token has valid provenance

---

### Checkpoint 4: Receiver Acceptance Signature

**Location:** After receiver accepts the transfer

**When:** Receiver signs to acknowledge receipt

**Code:**
```go
// Receiver signs the received token block
receiverDC, err := c.SetupDID(reqID, receiverDID)
err = tokenBlock.UpdateSignature(receiverDC)
```

**What's Verified:**
- ✅ Receiver's signature on new block
- ✅ Receiver accepts ownership

**Purpose:** Proves receiver acknowledged and accepted the transfer

---

### Checkpoint 5: Smart Contract Signature

**Location:** `contract/contract.go:394-449`

**When:** Before initiating consensus

**Code:**
```go
func (c *Contract) UpdateSignature(dc did.DIDCrypto) error {
    hash, err := c.GetHash()
    ssig, psig, err := dc.Sign(hash)

    // Store sender's signature in contract
    c.sm[SCShareSignatureKey][did] = util.HexToStr(ssig)
    c.sm[SCKeySignatureKey][did] = util.HexToStr(psig)

    return nil
}
```

**What's Verified:**
- ✅ Sender signed the smart contract
- ✅ Contract parameters are authorized

**Purpose:** Binds sender to the contract terms

---

## Summary: All Verification Points

```
Token Transfer Flow:

1. ✅ SENDER SIGNS CONTRACT
   └─> contract.UpdateSignature(senderDID)
       └─> Verifies: Sender authorizes transfer

2. ✅ SENDER SIGNATURE VERIFIED
   └─> ValidateTokenChainBlock()
       └─> Verifies: Sender's NLSS + ECDSA signatures

3. ✅ QUORUM SIGNING
   └─> Each quorum signs token chain block
       └─> Verifies: Quorum participation

4. ✅ QUORUM SIGNATURES VERIFIED
   └─> For each quorum: NlssVerify() or PvtVerify()
       └─> Verifies: All quorums agree

5. ✅ PREVIOUS BLOCK SIGNATURES
   └─> Validate entire token chain history
       └─> Verifies: Valid provenance

6. ✅ RECEIVER ACCEPTANCE
   └─> Receiver signs new block
       └─> Verifies: Receiver acknowledges

7. ✅ COMMIT TRANSACTION
   └─> Add to blockchain with all signatures
       └─> Verifies: Immutable record
```

---

## API Flow Diagrams

### Flow 1: Initiate Transfer (All DID Modes)

```
Client                Node                      DID Module
  │                    │                            │
  │ POST /api/         │                            │
  │ initiate-rbt-      │                            │
  │ transfer           │                            │
  │───────────────────>│                            │
  │                    │                            │
  │                    │ 1. Validate request        │
  │                    │ 2. AddWebReq(req)          │
  │                    │    (create channel)        │
  │                    │                            │
  │                    │ 3. SetupDID(reqID, sender) │
  │                    │───────────────────────────>│
  │                    │                            │
  │                    │    InitDIDBasic() or       │
  │                    │    InitDIDWallet() or      │
  │                    │    InitDIDLite()           │
  │                    │<───────────────────────────│
  │                    │                            │
  │                    │ 4. Launch goroutine:       │
  │                    │    InitiateRBTTransfer()   │
  │                    │                            │
  │                    │ 5. didResponse(req, reqID) │
  │<───────────────────│                            │
  │                    │                            │
  │ Response:          │                            │
  │ {id, status, msg}  │                            │
  │                    │                            │
```

### Flow 2: BasicDID Password Flow

```
Client                Node                  BasicDID Module
  │                    │                            │
  │ (waiting...)       │ Sign(hash)                 │
  │                    │───────────────────────────>│
  │                    │                            │
  │                    │   getPassword()            │
  │                    │   │                        │
  │                    │   │ Password request       │
  │                    │   │ sent to OutChan        │
  │                    │<──┘                        │
  │                    │                            │
  │  didResponse()     │                            │
  │  polling OutChan   │                            │
  │<───────────────────│                            │
  │                    │                            │
  │ Response:          │                            │
  │ "Password needed"  │                            │
  │                    │                            │
  │ POST /api/         │                            │
  │ signature-response │                            │
  │ {id, password}     │                            │
  │───────────────────>│                            │
  │                    │                            │
  │                    │ Password sent to InChan    │
  │                    │───────────────────────────>│
  │                    │                            │
  │                    │   Decrypt pvtKey           │
  │                    │   Sign with ECDSA          │
  │                    │   Return signature         │
  │                    │<───────────────────────────│
  │                    │                            │
  │                    │ Continue consensus...      │
  │                    │                            │
```

### Flow 3: WalletDID Signature Flow

```
Client      Node            WalletDID         Wallet App
  │          │                  │                  │
  │          │ Sign(hash)       │                  │
  │          │─────────────────>│                  │
  │          │                  │                  │
  │          │   getSignature() │                  │
  │          │   │              │                  │
  │          │   │ Request sent │                  │
  │          │   │ to OutChan   │                  │
  │          │<──┘              │                  │
  │          │                  │                  │
  │<─────────│                  │                  │
  │          │                  │                  │
  │ Response:│                  │                  │
  │ "Signature needed"          │                  │
  │ {id, hash, mode}            │                  │
  │          │                  │                  │
  │ Show "Sign on Wallet" UI    │                  │
  │──────────┼──────────────────┼─────────────────>│
  │          │                  │                  │
  │          │                  │                  │ User
  │          │                  │                  │ Confirms
  │          │                  │                  │
  │          │                  │                  │ Load
  │          │                  │                  │ pvtShare
  │          │                  │                  │
  │          │                  │                  │ Extract
  │          │                  │                  │ bits
  │          │                  │                  │
  │          │                  │                  │ Sign with
  │          │                  │                  │ ECDSA
  │          │                  │                  │
  │ POST /api/signature-response│                  │
  │ {id, signature: {pixels, signature}}          │
  │─────────>│                  │                  │
  │          │                  │                  │
  │          │ Signature sent   │                  │
  │          │ to InChan        │                  │
  │          │─────────────────>│                  │
  │          │                  │                  │
  │          │   Return signature                  │
  │          │<─────────────────│                  │
  │          │                  │                  │
  │          │ Continue consensus...               │
  │          │                  │                  │
```

---

## Key Takeaways

1. **Two Signature Modes:**
   - NLSS (NlssVersion) - Quantum-resistant, 32 bytes + ECDSA
   - BIP (BIPVersion) - Industry-standard, ECDSA only

2. **Two Verification Modes:**
   - NlssVerify() - Dual-layer (NLSS + ECDSA)
   - PvtVerify() - Single-layer (ECDSA/secp256k1)

3. **Asynchronous Flow:**
   - API returns immediately with reqID
   - Background process requests signature/password
   - Client responds via /api/signature-response
   - Final result delivered via channel polling

4. **WalletDID Advantages:**
   - Private keys never leave wallet
   - User confirmation on device
   - Air-gapped security
   - Hardware wallet support

5. **Multiple Verification Points:**
   - Sender signature (contract)
   - Sender signature (block)
   - Quorum signatures (consensus)
   - Historical signatures (provenance)
   - Receiver signature (acceptance)

6. **Channel-Based Communication:**
   - OutChan: Node → Client (requests)
   - InChan: Client → Node (responses)
   - Enables async, non-blocking operations

This architecture provides flexible, secure transaction signing across multiple DID modes while maintaining backward compatibility and quantum resistance.
