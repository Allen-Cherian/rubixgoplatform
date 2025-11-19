# Sender Signature Points in Token Transfer

Complete analysis of **exactly where and when** the sender puts their signature during a token transfer in Rubix Platform.

---

## Executive Summary

During a token transfer, the **sender signs at 2 critical points**:

1. **Smart Contract Signature** - Signs the transfer contract (sender authorization)
2. **Token Chain Block Signature** - Signs each token's blockchain block (ownership transfer)

---

## Point 1: Smart Contract Signature

### Location
**File:** `core/transfer.go:403`
**Function:** `initiateRBTTransfer()`

### Code
```go
contractType := getContractType(reqID, req, tis, isSelfRBTTransfer)
sc := contract.CreateNewContract(contractType)

// SIGNATURE POINT 1: Sign the smart contract
err = sc.UpdateSignature(dc)
if err != nil {
    c.log.Error(err.Error())
    resp.Message = err.Error()
    return resp
}
```

### What Gets Signed
The **smart contract** containing:
- Sender DID
- Receiver DID
- Token list (token IDs and values)
- Total RBT amount
- Comment/memo
- Transaction metadata

### Implementation Details

**File:** `contract/contract.go:394-449`

```go
func (c *Contract) UpdateSignature(dc did.DIDCrypto) error {
    did := dc.GetDID()

    // Step 1: Get hash of the entire smart contract
    hash, err := c.GetHash()
    if err != nil {
        return fmt.Errorf("Failed to get hash of smart contract, " + err.Error())
    }

    // Step 2: SIGN THE CONTRACT HASH
    // This is where the sender signature happens!
    ssig, psig, err := dc.Sign(hash)
    //                    ^^^^^^^^^^
    //                    Calls sender's DID Sign() method
    if err != nil {
        return fmt.Errorf("Failed to get signature, " + err.Error())
    }

    // Step 3: Store NLSS signature in contract
    if c.sm[SCShareSignatureKey] == nil {
        ksm := make(map[string]interface{})
        ksm[did] = util.HexToStr(ssig)  // Store 32 bytes (NLSS share)
        c.sm[SCShareSignatureKey] = ksm
    } else {
        ksm := c.sm[SCShareSignatureKey].(map[string]interface{})
        ksm[did] = util.HexToStr(ssig)
        c.sm[SCShareSignatureKey] = ksm
    }

    // Step 4: Store ECDSA signature in contract
    if c.sm[SCKeySignatureKey] == nil {
        ksm := make(map[string]interface{})
        ksm[did] = util.HexToStr(psig)  // Store ECDSA signature
        c.sm[SCKeySignatureKey] = ksm
    } else {
        ksm := c.sm[SCKeySignatureKey].(map[string]interface{})
        ksm[did] = util.HexToStr(psig)
        c.sm[SCKeySignatureKey] = ksm
    }

    return nil
}
```

### Signature Components

**For NLSS Modes (BasicDID, WalletDID, etc.):**
```
ssig = 32 bytes (256 bits from pvtShare)
psig = ECDSA-P256 signature over SHA3-256(ssig)
```

**For BIP Modes (LiteDID):**
```
ssig = empty []byte
psig = secp256k1 signature over contract hash
```

### What This Signature Proves
✅ Sender authorizes this specific transfer
✅ Sender agrees to transfer these specific tokens
✅ Sender agrees to these specific receivers
✅ Sender cannot deny initiating this transaction

### When This Happens
**Timing:** Immediately after token gathering, **BEFORE** contacting quorums

**User Interaction:**
- For **BasicDID**: User prompted for password
- For **WalletDID**: User prompted to sign on wallet device
- For **StandardDID**: Remote signer contacted
- For **LiteDID**: Password requested (or wallet fallback)

---

## Point 2: Token Chain Block Signatures

### Location
**File:** `core/quorum_initiator.go:629`
**Function:** `initiateConsensus()`

### Code
```go
// Get the token info from smart contract
ti := sc.GetTransTokenInfo()

// SIGNATURE POINT 2: Create and sign token chain blocks
nb, err := c.pledgeQuorumToken(cr, sc, tid, dc)
//                                           ^^
//                                     DID crypto for signing
if err != nil {
    c.log.Error("failed to create token chain block", "err", err)
    return nil, nil, nil, err
}
```

### What Gets Signed
A **token chain block** for EACH token being transferred, containing:
- Token ID
- Previous block hash (chain link)
- New owner DID (receiver)
- Transaction ID
- Transaction type (transfer)
- Timestamp
- Quorum signatures (added later)
- **Sender's signature** (proves sender authorized this specific block)

### Implementation Chain

#### Step 2.1: pledgeQuorumToken()

**File:** `core/quorum_initiator.go:2700-2800`

```go
func (c *Core) pledgeQuorumToken(cr *ConensusRequest, sc *contract.Contract, tid string, dc did.DIDCrypto) (*block.Block, error) {
    pd := c.pd[cr.ReqID]
    cs := c.quorumRequest[cr.ReqID]

    // Extract token info
    ti := sc.GetTransTokenInfo()

    // Build token chain block structure
    var tcb block.TokenChainBlock
    tcb.TransactionType = block.TokenTransferredType
    tcb.TransInfo = sc.GetTransInfo()
    tcb.TokensData = make([]block.TokenData, 0)

    for i := range ti {
        // For each token, get its latest block
        lb := c.w.GetLatestTokenBlock(ti[i].Token, ti[i].TokenType)
        pbid, err := lb.GetBlockID(ti[i].Token)

        // Create new token data
        var td block.TokenData
        td.Token = ti[i].Token
        td.TokenType = ti[i].TokenType
        td.OwnerDID = sc.GetReceiverDID()  // New owner
        td.PreviousBlock = pbid             // Link to previous block

        tcb.TokensData = append(tcb.TokensData, td)
    }

    // Create the token chain block config
    ctcb := block.CreateTCB{
        TokenChainType: block.TCTType,
        TransactionID:  tid,
    }

    // CREATE NEW BLOCK
    nb := block.CreateNewBlock(ctcb, &tcb)
    if nb == nil {
        c.log.Error("Failed to create new token chain block - qrm init")
        return nil, fmt.Errorf("failed to create new token chain block - qrm init")
    }

    blk := nb.GetBlock()

    // Request quorum signatures (each quorum signs this block)
    for k := range pd.PledgedTokens {
        p := cs.P[k]  // Peer connection to quorum

        sr := SignatureRequest{
            TokenChainBlock: blk,
        }
        var srep SignatureReply

        // Request quorum to sign
        err := p.SendJSONRequest("POST", APISignatureRequest, nil, &sr, &srep, true)
        if err != nil {
            return nil, fmt.Errorf("failed to get signature from the quorum")
        }

        // Add quorum signature to block
        err = nb.ReplaceSignature(k, srep.Signature)
        if err != nil {
            return nil, err
        }
    }

    // SENDER SIGNS THE BLOCK HERE (implicit)
    // The block already contains sender info and will be signed
    // when added to the token chain

    return nb, nil
}
```

#### Step 2.2: Sender Signature Added to Block

**Important:** The sender's signature is added to the token chain block in **multiple scenarios**:

##### Scenario A: During Token Transfer Commit

**File:** `core/quorum_initiator.go:3019-3034`

```go
func (c *Core) commitRBTToken(...) error {
    // Build token chain block
    var tcb block.TokenChainBlock
    tcb.TransactionType = block.TokenTransferredType
    tcb.TransInfo = transInfo
    tcb.TokensData = tds

    ctcb := block.CreateTCB{
        TokenChainType: block.TCTType,
        TransactionID:  transactionID,
    }

    // Create new block
    nb := block.CreateNewBlock(ctcb, &tcb)
    if nb == nil {
        return fmt.Errorf("Failed to create new token chain block")
    }

    // SENDER SIGNS THE BLOCK
    err = nb.UpdateSignature(didCryptoLib)
    //        ^^^^^^^^^^^^^^^
    //        This adds sender's signature!
    if err != nil {
        return fmt.Errorf("Failed to update signature to block")
    }

    // Save block to token chain
    err = c.w.CreateTokenBlock(nb)
    if err != nil {
        return fmt.Errorf("Failed to update token chain block")
    }

    return nil
}
```

##### Scenario B: During TokensTransferred()

**File:** `core/wallet/token.go` (called from quorum_initiator.go:895)

```go
// After consensus, update token chain
err = c.w.TokensTransferred(sc.GetSenderDID(), ti, nb, rp.IsLocal(), sr.PinningServiceMode)
```

**Inside TokensTransferred():**

**File:** `core/wallet/token.go`

```go
func (w *Wallet) TokensTransferred(did string, ti []contract.TokenInfo, nb *block.Block, isLocal bool, isPinningServiceMode bool) error {
    for _, t := range ti {
        // Update token ownership
        tk := Token{
            TokenID:    t.Token,
            DID:        did,
            TokenValue: t.TokenValue,
        }

        // Get latest block
        lb := w.GetLatestTokenBlock(t.Token, t.TokenType)

        // The block 'nb' already contains sender's signature
        // from the smart contract signing step

        // Add block to token chain
        err := w.AddTokenBlock(t.Token, nb)
        if err != nil {
            return err
        }

        // Update token status
        err = w.UpdateTokenStatus(tk.TokenID, wallet.TokenIsTransferred)
        if err != nil {
            return err
        }
    }
    return nil
}
```

### Block Signature Details

**File:** `block/block.go:441-467`

```go
func (b *Block) UpdateSignature(dc didmodule.DIDCrypto) error {
    did := dc.GetDID()

    // Step 1: Get hash of the entire block
    h, err := b.GetHash()
    if err != nil {
        return fmt.Errorf("failed to get hash")
    }

    // Step 2: SIGN THE BLOCK HASH (using PvtSign - ECDSA only)
    sb, err := dc.PvtSign([]byte(h))
    //            ^^^^^^^^
    //            Signs with private key only (no NLSS for block signatures)
    if err != nil {
        return fmt.Errorf("failed to get did signature, " + err.Error())
    }

    sig := util.HexToStr(sb)

    // Step 3: Add signature to block
    ksmi, ok := b.bm[TCSignatureKey]
    if !ok {
        // Create new signature map
        ksm := make(map[string]interface{})
        ksm[did] = sig
        b.bm[TCSignatureKey] = ksm
        return b.blkEncode()
    }

    // Add to existing signature map
    ksm := ksmi.(map[string]interface{})
    ksm[did] = sig
    b.bm[TCSignatureKey] = ksm
    return b.blkEncode()
}
```

**Important Note:** Block signatures use **PvtSign()** which is:
- **ECDSA-P256** signature for BasicDID/WalletDID/StandardDID
- **secp256k1** signature for LiteDID
- **No NLSS component** (unlike smart contract signatures)

### Signature Components for Token Blocks

**For all DID modes:**
```
Signature = ECDSA/secp256k1 signature over SHA3-256(block_hash)
```

**Block contains:**
- Sender DID (in signatures map)
- Receiver DID (in token data)
- All token IDs being transferred
- Previous block IDs (chain continuity)
- Quorum signatures (added by quorums)
- Transaction ID
- Timestamp

### What These Signatures Prove
✅ Sender authorizes transfer of each specific token
✅ Sender acknowledges previous block in chain (provenance)
✅ Sender confirms new owner is receiver
✅ Immutable record of ownership transfer

---

## Complete Signature Flow Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                   TOKEN TRANSFER FLOW                        │
└─────────────────────────────────────────────────────────────┘

1. API Request
   └─> /api/initiate-rbt-transfer

2. Gather Tokens
   └─> Lock tokens from sender's wallet

3. Create Smart Contract
   └─> contract.CreateNewContract()
       ├─ Contract contains:
       │  ├─ Sender DID
       │  ├─ Receiver DID
       │  ├─ Token list
       │  └─ Amount, comment, etc.

4. ✅ SIGNATURE POINT 1: Sign Smart Contract
   └─> sc.UpdateSignature(dc)
       │
       ├─> Get contract hash
       │
       ├─> dc.Sign(hash)
       │   │
       │   ├─ BasicDID: Load pvtShare, extract 256 bits, sign
       │   ├─ WalletDID: Request wallet to sign
       │   ├─ StandardDID: Request remote signer
       │   └─ LiteDID: Sign with secp256k1
       │
       ├─> Store signatures in contract:
       │   ├─ SCShareSignatureKey[senderDID] = nlss_share (32 bytes)
       │   └─ SCKeySignatureKey[senderDID] = ecdsa_signature
       │
       └─> Contract now contains sender's authorization

5. Initiate Consensus
   └─> c.initiateConsensus(cr, sc, dc)

6. ✅ SIGNATURE POINT 2: Create Token Chain Blocks
   └─> nb, err := c.pledgeQuorumToken(cr, sc, tid, dc)
       │
       ├─> For each token:
       │   ├─ Get latest block (previous block)
       │   ├─ Create new block:
       │   │   ├─ Token ID
       │   │   ├─ Previous block hash
       │   │   ├─ New owner (receiver DID)
       │   │   ├─ Transaction ID
       │   │   └─ Transaction type (transfer)
       │   │
       │   └─> Block structure created
       │
       ├─> Request quorum signatures
       │   └─> Each quorum signs the block
       │
       └─> Sender signature added during commit:
           └─> nb.UpdateSignature(dc)
               │
               ├─> Get block hash
               │
               ├─> dc.PvtSign(block_hash)
               │   └─ Signs with ECDSA/secp256k1 (no NLSS)
               │
               └─> Add signature to block:
                   └─ TCSignatureKey[senderDID] = ecdsa_signature

7. Contact Receiver
   └─> Send tokens and block to receiver

8. Quorum Pledge Finality
   └─> Confirm all quorums pledged correctly

9. Commit Transaction
   └─> Add blocks to blockchain
   └─> Update token ownership
   └─> Mark transaction complete

10. Response to Client
    └─> "Transfer finished successfully"
```

---

## Summary Table

| Signature Point | What Gets Signed | Signature Type | Purpose | When |
|-----------------|------------------|----------------|---------|------|
| **Point 1: Smart Contract** | Entire contract (sender, receiver, tokens, amount) | NLSS + ECDSA (or BIP) | Sender authorizes transfer | Before consensus |
| **Point 2: Token Blocks** | Each token's new blockchain block | ECDSA only (no NLSS) | Record ownership transfer | During consensus |

---

## Key Differences

### Smart Contract Signature vs Token Block Signature

| Aspect | Smart Contract | Token Block |
|--------|----------------|-------------|
| **What** | Transfer authorization | Ownership record |
| **Signature Method** | `dc.Sign()` (full NLSS+ECDSA) | `dc.PvtSign()` (ECDSA only) |
| **NLSS Component** | ✅ Yes (32 bytes) | ❌ No |
| **ECDSA Component** | ✅ Yes | ✅ Yes |
| **Quantum Resistant** | ✅ Yes (NLSS) | ❌ No (ECDSA only) |
| **Stored Where** | Smart contract block | Token chain blocks |
| **Verifiers** | Quorums, receiver | Future token holders |
| **Frequency** | Once per transaction | Once per token |

---

## Signature Verification During Transfer

### Where Sender Signatures Are Verified

#### Verification Point 1: Token Chain Validation

**File:** `core/token_chain_validation.go:563-576`

```go
// Verify sender's signature on the smart contract/block
if senderDIDType == did.LiteDIDMode {
    // BIP verification
    response.Status, err = didCrypto.PvtVerify(
        []byte(senderSign.Hash),
        util.StrToHex(senderSign.PrivateSign)
    )
} else {
    // NLSS verification
    response.Status, err = didCrypto.NlssVerify(
        senderSign.Hash,
        util.StrToHex(senderSign.NLSSShare),
        util.StrToHex(senderSign.PrivateSign)
    )
}
```

**What's Verified:**
- ✅ Sender's NLSS signature (if applicable)
- ✅ Sender's ECDSA signature
- ✅ Hash matches the block

#### Verification Point 2: Receiver Acceptance

**When receiver accepts tokens:**
- Receiver verifies sender's signature on token blocks
- Receiver verifies all quorum signatures
- Receiver verifies chain continuity (previous blocks)

---

## User Experience During Signing

### BasicDIDMode Flow

```
1. User initiates transfer
   └─> POST /api/initiate-rbt-transfer

2. Node responds immediately
   └─> {"id": "abc123", "status": false, "message": "Processing..."}

3. Node needs to sign contract
   └─> Internal: sc.UpdateSignature(dc)
       └─> Calls getPassword()
           └─> Sends password request to client

4. Client receives password request
   └─> {"status": true, "message": "Password needed", "result": {"id": "abc123"}}

5. User enters password
   └─> POST /api/signature-response
       └─> {"id": "abc123", "password": "user_password"}

6. Node receives password
   └─> Decrypts pvtKey
   └─> Signs contract (Point 1)
   └─> Continues with consensus

7. Node creates token blocks
   └─> Signs each block (Point 2)
   └─> Uses same password (cached)

8. Transfer completes
   └─> {"status": true, "message": "Transfer finished successfully"}
```

### WalletDIDMode Flow

```
1. User initiates transfer
   └─> POST /api/initiate-rbt-transfer

2. Node responds immediately
   └─> {"id": "abc123", "status": false, "message": "Processing..."}

3. Node needs to sign contract
   └─> Internal: sc.UpdateSignature(dc)
       └─> Calls getSignature()
           └─> Sends signature request to wallet

4. Client receives signature request
   └─> {"status": true, "message": "Signature needed",
        "result": {"id": "abc123", "hash": "...", "mode": 2}}

5. Wallet app displays confirmation
   └─> "Sign transaction: Transfer 10 RBT to bafybmi..."
   └─> User confirms on wallet device

6. Wallet signs
   └─> Loads pvtShare from secure element
   └─> Extracts 256 bits
   └─> Signs with ECDSA

7. Wallet sends signature
   └─> POST /api/signature-response
       └─> {"id": "abc123", "signature": {"pixels": "...", "signature": "..."}}

8. Node receives signature
   └─> Signs contract (Point 1)
   └─> Continues with consensus

9. Node needs to sign token blocks
   └─> Requests wallet signature again (Point 2)
   └─> Wallet signs block hash

10. Transfer completes
    └─> {"status": true, "message": "Transfer finished successfully"}
```

---

## Important Notes

### 1. Password/Signature Caching

**BasicDID:**
- Password requested once
- Cached for the duration of the request
- Used for both contract and block signing
- Cleared after transaction completes

**WalletDID:**
- Signature requested twice:
  1. For smart contract (NLSS + ECDSA)
  2. For token blocks (ECDSA only)
- User must confirm each signature on wallet device
- No caching (security feature)

### 2. Multiple Token Signatures

If transferring **N tokens**, sender signs:
- **1× Smart Contract** (authorizes entire transfer)
- **1× Token Chain Block** (contains all N tokens)

**Not N separate blocks!** All tokens in same transaction share one block.

### 3. Signature Immutability

Once signed:
- ✅ Smart contract signature is permanent
- ✅ Token block signatures are permanent
- ❌ Cannot be modified or revoked
- ❌ Cannot transfer tokens back without new transaction

### 4. Failure Handling

If signing fails:
- Transaction aborted immediately
- Locked tokens released
- No blockchain changes
- Error returned to client

---

## Conclusion

The sender puts their signature at **exactly 2 points** during token transfer:

1. **Smart Contract Signature** (`sc.UpdateSignature(dc)`) - Authorizes the transfer
2. **Token Block Signatures** (`nb.UpdateSignature(dc)`) - Records ownership change

Both signatures are cryptographically binding and create an immutable audit trail of the transaction.
