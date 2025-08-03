# FT Transaction History Testing Guide

## Prerequisites

1. **Build the binary**
   ```bash
   cd /path/to/rubixgoplatform
   go build -o rubixgoplatform
   ```

2. **Setup multiple nodes** (if testing between different nodes)
   ```bash
   # Node 1
   ./rubixgoplatform run -p 20000 -testNet

   # Node 2 (different terminal)
   ./rubixgoplatform run -p 20001 -testNet
   ```

## Testing Commands

### 1. Create DIDs on both nodes
```bash
# Node 1
curl -X POST http://localhost:20000/api/createdid

# Node 2
curl -X POST http://localhost:20001/api/createdid
```

### 2. Create FT Token
```bash
# On Node 1 - Create FT with 1000 tokens
curl -X POST http://localhost:20000/api/create-ft \
  -H "Content-Type: application/json" \
  -d '{
    "FTName": "TestFT",
    "TokenCount": 1000,
    "DID": "<YOUR_DID_FROM_NODE1>"
  }'
```

### 3. Transfer FT Tokens
```bash
# Transfer 100 tokens from Node 1 to Node 2
curl -X POST http://localhost:20000/api/transfer-ft \
  -H "Content-Type: application/json" \
  -d '{
    "FTName": "TestFT",
    "TokenCount": 100,
    "ReceiverDID": "<DID_FROM_NODE2>",
    "Comment": "Test transfer 100 tokens"
  }'
```

### 4. Check FT Transaction History

#### Get all FT transactions for a DID
```bash
# Check sender's history
curl -X GET "http://localhost:20000/api/get-ft-txn-details?did=<YOUR_DID_FROM_NODE1>"

# Check receiver's history
curl -X GET "http://localhost:20001/api/get-ft-txn-details?did=<DID_FROM_NODE2>"
```

#### Get FT transactions by sender
```bash
curl -X GET "http://localhost:20000/api/get-ft-txn-by-sender?sender=<YOUR_DID_FROM_NODE1>"
```

#### Get FT transactions by receiver
```bash
curl -X GET "http://localhost:20001/api/get-ft-txn-by-receiver?receiver=<DID_FROM_NODE2>"
```

### 5. Test Large Transfers (Memory Optimization)
```bash
# Create FT with 6000 tokens
curl -X POST http://localhost:20000/api/create-ft \
  -H "Content-Type: application/json" \
  -d '{
    "FTName": "LargeFT",
    "TokenCount": 6000,
    "DID": "<YOUR_DID_FROM_NODE1>"
  }'

# Transfer 5000 tokens (tests sender optimization)
curl -X POST http://localhost:20000/api/transfer-ft \
  -H "Content-Type: application/json" \
  -d '{
    "FTName": "LargeFT", 
    "TokenCount": 5000,
    "ReceiverDID": "<DID_FROM_NODE2>",
    "Comment": "Large transfer test"
  }'
```

### 6. Test Token History Persistence

```bash
# Step 1: Check initial history after transfer
curl -X GET "http://localhost:20000/api/get-ft-txn-details?did=<YOUR_DID_FROM_NODE1>"

# Step 2: Transfer the tokens away from Node 2 to Node 1 (or another node)
curl -X POST http://localhost:20001/api/transfer-ft \
  -H "Content-Type: application/json" \
  -d '{
    "FTName": "TestFT",
    "TokenCount": 50,
    "ReceiverDID": "<YOUR_DID_FROM_NODE1>",
    "Comment": "Transfer back"
  }'

# Step 3: Check history again - should still show original transfer
curl -X GET "http://localhost:20001/api/get-ft-txn-details?did=<DID_FROM_NODE2>"
```

## Expected Results

### Transaction History Response Format
```json
{
  "status": true,
  "result": [
    {
      "transaction_id": "QmXXXXXXXXXXXXXXXXXXXXXXXXXX",
      "mode": 5,  // FTTransferMode
      "transaction_type": 7,  // FT transfer
      "amount": 100,  // Number of tokens
      "sender_did": "bafybmi...",
      "receiver_did": "bafybmi...",
      "comment": "Test transfer 100 tokens",
      "date_time": "2025-08-03T10:00:00Z",
      "status": true,
      "tokens": [
        {
          "creator_did": "bafybmi...",
          "ft_name": "TestFT",
          "count": 100
        }
      ]
    }
  ]
}
```

## Monitoring and Verification

### 1. Check logs for transaction history storage
```bash
# Look for these log messages:
grep "FT transaction token metadata stored" node.log
grep "Transaction history added" node.log
```

### 2. Monitor memory usage during large transfers
```bash
# While transfer is running
ps aux | grep rubixgoplatform
# or
top -p $(pgrep rubixgoplatform)
```

### 3. Verify explorer synchronization
```bash
# Look for log indicating explorer submission completed before confirmation
grep "Explorer submission completed" node.log
grep "Sending confirmation request to receiver" node.log
```

## Troubleshooting

### If transaction history is not saved:
1. Check if the table exists (backward compatibility)
2. Look for error logs: `grep "Failed to store FT transaction token metadata" node.log`

### If large transfers fail:
1. Check memory usage during transfer
2. Verify sender optimization is triggered (>1000 tokens)
3. Look for: `grep "Using optimized sender for large FT transfer" node.log`

### If confirmation delays seem wrong:
1. Check calculated delays in logs
2. Verify explorer submission completes first
3. Look for: `grep "Calculated confirmation delay" node.log`

## Advanced Testing

### Test multiple concurrent transfers
```bash
# Run multiple transfers in parallel
for i in {1..5}; do
  curl -X POST http://localhost:20000/api/transfer-ft \
    -H "Content-Type: application/json" \
    -d "{
      \"FTName\": \"TestFT\",
      \"TokenCount\": 10,
      \"ReceiverDID\": \"<DID_FROM_NODE2>\",
      \"Comment\": \"Concurrent transfer $i\"
    }" &
done
wait
```

### Test with different token counts
```bash
# Test optimization thresholds
for count in 100 500 1000 2000 5000; do
  echo "Testing with $count tokens..."
  # Create and transfer tokens
done
```