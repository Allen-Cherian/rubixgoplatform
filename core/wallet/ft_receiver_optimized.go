package wallet

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/rubixchain/rubixgoplatform/block"
	"github.com/rubixchain/rubixgoplatform/contract"
	"github.com/rubixchain/rubixgoplatform/core/model"
	"github.com/rubixchain/rubixgoplatform/util"
	ipfsnode "github.com/ipfs/go-ipfs-api"
)

// OptimizedFTTokensReceived processes received FT tokens with batch IPFS Get operations
func (w *Wallet) OptimizedFTTokensReceived(did string, ti []contract.TokenInfo, b *block.Block, senderPeerId string, receiverPeerId string, ipfsShell *ipfsnode.Shell, ftInfo FTToken) ([]string, error) {
	startTime := time.Now()
	w.l.Lock()
	defer w.l.Unlock()
	
	w.log.Info("Starting optimized FT token receive", 
		"ft_count", len(ti),
		"ft_name", ftInfo.FTName,
		"transaction_id", b.GetTid(),
		"sender", senderPeerId,
		"receiver", receiverPeerId)
	
	// Create token block first
	err := w.CreateTokenBlock(b)
	if err != nil {
		w.log.Error("Failed to create token block", "error", err)
		return nil, err
	}

	// Phase 1: Prepare token state hashes
	updatedtokenhashes := make([]string, 0)
	tokenHashMap := make(map[string]string)
	providerMaps := make([]model.TokenProviderMap, 0, len(ti))
	
	addStart := time.Now()
	for _, info := range ti {
		t := info.Token
		b := w.GetLatestTokenBlock(info.Token, info.TokenType)
		blockId, _ := b.GetBlockID(t)
		tokenIDTokenStateData := t + blockId
		tokenIDTokenStateBuffer := bytes.NewBuffer([]byte(tokenIDTokenStateData))
		tokenIDTokenStateHash, tpm, _ := w.AddWithProviderMap(tokenIDTokenStateBuffer, did, OwnerRole)
		updatedtokenhashes = append(updatedtokenhashes, tokenIDTokenStateHash)
		tokenHashMap[t] = tokenIDTokenStateHash
		// Fill in extra fields for pinning
		tpm.FuncID = PinFunc
		tpm.TransactionID = b.GetTid()
		tpm.Sender = senderPeerId + "." + b.GetSenderDID()
		tpm.Receiver = receiverPeerId + "." + b.GetReceiverDID()
		tpm.TokenValue = info.TokenValue
		providerMaps = append(providerMaps, tpm)
	}
	
	w.log.Info("FT token state addition completed", 
		"count", len(ti),
		"duration", time.Since(addStart))

	// Phase 2: Identify FT tokens that need to be downloaded
	getRequests := make([]GetRequest, 0)
	tokenIndexMap := make(map[string]int)
	downloadDirs := make([]string, 0)
	
	checkStart := time.Now()
	for i, tokenInfo := range ti {
		var FTInfo FTToken
		err := w.s.Read(FTTokenStorage, &FTInfo, "token_id=?", tokenInfo.Token)
		if err != nil || FTInfo.TokenID == "" {
			// Token doesn't exist, need to download
			dir := util.GetRandString()
			if err := util.CreateDir(dir); err != nil {
				w.log.Error("Failed to create directory", 
					"dir", dir,
					"error", err)
				// Cleanup previously created dirs
				for _, d := range downloadDirs {
					os.RemoveAll(d)
				}
				return nil, fmt.Errorf("failed to create directory: %v", err)
			}
			downloadDirs = append(downloadDirs, dir)
			
			getRequests = append(getRequests, GetRequest{
				Hash:  tokenInfo.Token,
				DID:   did,
				Role:  OwnerRole,
				Path:  dir,
				Index: i,
			})
			tokenIndexMap[tokenInfo.Token] = i
		}
	}
	
	w.log.Info("FT token existence check completed", 
		"existing_tokens", len(ti)-len(getRequests),
		"tokens_to_download", len(getRequests),
		"duration", time.Since(checkStart))

	// Phase 3: Batch download FT tokens with all-or-nothing semantics
	var downloadProviderMaps []model.TokenProviderMap
	if len(getRequests) > 0 {
		downloadStart := time.Now()
		downloadProviderMaps, err = w.BatchGetWithProviderMaps(getRequests)
		if err != nil {
			w.log.Error("FT batch download failed, cleaning up", 
				"error", err)
			
			// Cleanup created directories
			for _, dir := range downloadDirs {
				if rmErr := os.RemoveAll(dir); rmErr != nil {
					w.log.Error("Failed to cleanup directory", 
						"dir", dir,
						"error", rmErr)
				}
			}
			
			// Ensure no orphaned DB records
			tokenIDs := make([]string, len(getRequests))
			for i, req := range getRequests {
				tokenIDs[i] = req.Hash
			}
			if cleanupErr := w.ensureNoOrphanedRecords(tokenIDs, b.GetTid()); cleanupErr != nil {
				w.log.Error("Failed to cleanup orphaned records", 
					"error", cleanupErr)
			}
			
			return nil, fmt.Errorf("failed to download FT tokens: %v", err)
		}
		
		w.log.Info("FT batch download completed successfully", 
			"count", len(getRequests),
			"duration", time.Since(downloadStart))
	}
	
	// Merge provider maps
	allProviderMaps := append(providerMaps, downloadProviderMaps...)

	// Phase 4: Process downloaded FT tokens
	processStart := time.Now()
	processedCount := 0
	
	for _, req := range getRequests {
		tokenInfo := ti[req.Index]
		
		// Get genesis block for owner info
		tt := tokenInfo.TokenType
		blk := w.GetGenesisTokenBlock(tokenInfo.Token, tt)
		if blk == nil {
			w.log.Error("Failed to get genesis block for Parent DID updation, invalid token chain",
				"token", tokenInfo.Token)
			continue
		}
		FTOwner := blk.GetOwner()
		
		// Create new FT token entry
		FTInfo := FTToken{
			TokenID:    tokenInfo.Token,
			TokenValue: tokenInfo.TokenValue,
			CreatorDID: FTOwner,
			FTName:     ftInfo.FTName,
			DID:        did,
			TokenStatus: TokenIsFree,
			TransactionID: b.GetTid(),
			TokenStateHash: tokenHashMap[tokenInfo.Token],
		}

		err = w.s.Write(FTTokenStorage, &FTInfo)
		if err != nil {
			w.log.Error("Failed to write FT token to db", 
				"token", tokenInfo.Token,
				"error", err)
			// Continue processing other tokens
			continue
		}
		
		// Cleanup the download directory
		os.RemoveAll(req.Path)
		processedCount++
	}
	
	// Phase 5: Process existing tokens (update status and pin)
	pinnedCount := 0
	
	for _, tokenInfo := range ti {
		// For tokens that already existed, just update status
		var FTInfo FTToken
		err := w.s.Read(FTTokenStorage, &FTInfo, "token_id=?", tokenInfo.Token)
		if err != nil {
			w.log.Error("Failed to read FT token for update", 
				"token", tokenInfo.Token,
				"error", err)
			continue
		}
		
		// Update token status
		FTInfo.FTName = ftInfo.FTName
		FTInfo.DID = did
		FTInfo.TokenStatus = TokenIsFree
		FTInfo.TransactionID = b.GetTid()
		FTInfo.TokenStateHash = tokenHashMap[tokenInfo.Token]

		err = w.s.Update(FTTokenStorage, &FTInfo, "token_id=?", tokenInfo.Token)
		if err != nil {
			w.log.Error("Failed to update FT token in db", 
				"token", tokenInfo.Token,
				"error", err)
			continue
		}
		
		senderAddress := senderPeerId + "." + b.GetSenderDID()
		receiverAddress := receiverPeerId + "." + b.GetReceiverDID()
		
		// Pin the token
		_, err = w.Pin(tokenInfo.Token, OwnerRole, did, b.GetTid(), senderAddress, receiverAddress, tokenInfo.TokenValue, true)
		if err != nil {
			w.log.Error("Failed to pin FT token", 
				"token", tokenInfo.Token,
				"error", err)
			continue
		}
		pinnedCount++
	}
	
	w.log.Info("FT token processing completed", 
		"downloaded", processedCount,
		"pinned", pinnedCount,
		"total", len(ti),
		"duration", time.Since(processStart))

	// Phase 6: Handle provider details
	providerStart := time.Now()
	if len(allProviderMaps) > 100 && w.asyncProviderMgr != nil {
		err := w.asyncProviderMgr.SubmitProviderDetails(allProviderMaps, b.GetTid())
		if err != nil {
			w.log.Error("Failed to submit provider details to async queue, falling back to sync", 
				"count", len(allProviderMaps),
				"error", err)
			goto syncProcessing
		}
		w.log.Info("FT provider details submitted for async processing", 
			"transaction_id", b.GetTid(),
			"count", len(allProviderMaps),
			"duration", time.Since(providerStart))
		
		w.log.Info("Optimized FT token receive completed", 
			"total_tokens", len(ti),
			"downloaded", len(getRequests),
			"total_duration", time.Since(startTime))
		
		return updatedtokenhashes, nil
	}

syncProcessing:
	// Batch write provider details synchronously
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := w.AddProviderDetailsBatch(allProviderMaps)
		if err == nil {
			w.log.Info("FT provider details batch write successful", 
				"count", len(allProviderMaps),
				"attempt", attempt+1,
				"duration", time.Since(providerStart))
			
			w.log.Info("Optimized FT token receive completed", 
				"total_tokens", len(ti),
				"downloaded", len(getRequests),
				"total_duration", time.Since(startTime))
			
			return updatedtokenhashes, nil
		}
		w.log.Error("Batch AddProviderDetails failed, retrying", 
			"attempt", attempt+1,
			"error", err)
		time.Sleep(backoff(attempt))
	}
	
	return nil, fmt.Errorf("failed to batch add provider details after retries")
}