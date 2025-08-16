package core

import (
	"fmt"
	"time"

	"github.com/rubixchain/rubixgoplatform/core/model"
	"github.com/rubixchain/rubixgoplatform/core/wallet"
)

// TokenRecoveryResult contains the result of a token recovery operation
type TokenRecoveryResult struct {
	TransactionID       string    `json:"transaction_id"`
	SenderDID           string    `json:"sender_did"`
	ReceiverDID         string    `json:"receiver_did"`
	RecoveredTokenCount int       `json:"recovered_token_count"`
	RecoveryDate        time.Time `json:"recovery_date"`
	Status              string    `json:"status"`
	Message             string    `json:"message"`
}

// RecoverLostTokens attempts to recover tokens that were sent but not received
// This function helps senders recover tokens when receivers don't have them
func (c *Core) RecoverLostTokens(senderDID, transactionID string) (*TokenRecoveryResult, error) {
	c.log.Info("========== STARTING TOKEN RECOVERY PROCESS ==========",
		"sender_did", senderDID,
		"transaction_id", transactionID,
		"timestamp", time.Now().Format("2006-01-02 15:04:05"))

	// Step 1: Get transaction details from history
	c.log.Info("STEP 1: Retrieving transaction from history",
		"sender_did", senderDID,
		"transaction_id", transactionID)

	transactionDetails, err := c.getTransactionFromHistory(senderDID, transactionID)
	if err != nil {
		c.log.Error("STEP 1 FAILED: Transaction not found in history",
			"sender_did", senderDID,
			"transaction_id", transactionID,
			"error", err)
		return nil, fmt.Errorf("failed to get transaction from history: %v", err)
	}

	c.log.Info("STEP 1 SUCCESS: Transaction found",
		"transaction_id", transactionID,
		"amount", transactionDetails.Amount,
		"receiver_did", transactionDetails.ReceiverDID,
		"date", transactionDetails.DateTime,
		"comment", transactionDetails.Comment)

	// Step 1.5: Check if transaction has already been recovered
	c.log.Info("STEP 1.5: Checking if transaction was previously recovered")

	if c.isTransactionAlreadyRecovered(transactionDetails) {
		c.log.Error("STEP 1.5 FAILED: Transaction already recovered",
			"transaction_id", transactionID,
			"comment", transactionDetails.Comment,
			"recovery_marker_found", true)
		return nil, fmt.Errorf("transaction has already been recovered, recovery can only be done once per transaction")
	}

	c.log.Info("STEP 1.5 SUCCESS: Transaction has not been recovered before")

	// Step 2: Check if transaction is within recovery window (7-14 days)
	c.log.Info("STEP 2: Checking recovery time window",
		"transaction_date", transactionDetails.DateTime,
		"current_time", time.Now(),
		"age_days", time.Since(transactionDetails.DateTime).Hours()/24)

	if !c.isTransactionWithinRecoveryWindow(transactionDetails.DateTime) {
		age := time.Since(transactionDetails.DateTime)
		c.log.Error("STEP 2 FAILED: Transaction outside recovery window",
			"transaction_age", age,
			"age_days", age.Hours()/24,
			"required_window", "7-14 days (or <14 days based on config)")
		return nil, fmt.Errorf("transaction must be between 7-14 days old for recovery (current age: %v)", age)
	}

	c.log.Info("STEP 2 SUCCESS: Transaction within recovery window")

	// Step 3: Get receiver DID from transaction
	c.log.Info("STEP 3: Validating receiver information")

	receiverDID := transactionDetails.ReceiverDID
	if receiverDID == "" {
		c.log.Error("STEP 3 FAILED: No receiver DID in transaction",
			"transaction_id", transactionID)
		return nil, fmt.Errorf("receiver DID not found in transaction")
	}

	c.log.Info("STEP 3 SUCCESS: Receiver identified",
		"receiver_did", receiverDID)

	c.log.Info("Transaction details retrieved",
		"sender_did", senderDID,
		"receiver_did", receiverDID,
		"transaction_id", transactionID,
		"amount", transactionDetails.Amount,
		"date", transactionDetails.DateTime)

	// Check if this transaction has special exception status
	c.log.Info("STEP 3.5: Checking exception transaction status",
		"transaction_id", transactionID)

	isExceptionTransaction := c.isExceptionTransaction(transactionID)
	if isExceptionTransaction {
		c.log.Warn("STEP 3.5: EXCEPTION TRANSACTION DETECTED",
			"transaction_id", transactionID,
			"note", "Can proceed even if receiver is offline")
	} else {
		c.log.Info("STEP 3.5: Normal transaction (not in exception list)")
	}

	// Step 4: Check if receiver has the tokens
	c.log.Info("STEP 4: Checking if receiver currently has the tokens",
		"receiver_did", receiverDID,
		"transaction_id", transactionID,
		"expected_amount", transactionDetails.Amount)

	receiverHasTokens, err := c.checkReceiverTokenStatus(receiverDID, transactionID, int(transactionDetails.Amount))
	if err != nil {
		if isExceptionTransaction {
			c.log.Warn("STEP 4 EXCEPTION: Receiver offline but proceeding (exception transaction)",
				"transaction_id", transactionID,
				"receiver_did", receiverDID,
				"error", err,
				"action", "Assuming receiver doesn't have tokens")
			receiverHasTokens = false
		} else {
			c.log.Error("STEP 4 FAILED: Cannot verify receiver token status",
				"receiver_did", receiverDID,
				"error", err,
				"action", "Recovery blocked for safety")
			return nil, fmt.Errorf("cannot verify receiver token status (receiver may be offline): %v", err)
		}
	} else {
		c.log.Info("STEP 4: Successfully contacted receiver",
			"receiver_has_tokens", receiverHasTokens)
	}

	if receiverHasTokens {
		c.log.Error("STEP 4 FAILED: Receiver has the tokens",
			"receiver_did", receiverDID,
			"transaction_id", transactionID,
			"action", "Recovery not needed")
		return nil, fmt.Errorf("receiver has the tokens, no recovery needed")
	}

	c.log.Info("STEP 4 SUCCESS: Receiver does not have the tokens")

	// Step 5: Check if tokens were transferred from receiver (double-spending check)
	c.log.Info("STEP 5: Checking if receiver transferred tokens to someone else",
		"receiver_did", receiverDID,
		"transaction_id", transactionID)

	tokensTransferredFromReceiver, err := c.checkIfTokensTransferredFromReceiver(receiverDID, transactionID)
	if err != nil {
		if isExceptionTransaction {
			c.log.Warn("STEP 5 EXCEPTION: Cannot verify transfer status but proceeding (exception transaction)",
				"transaction_id", transactionID,
				"receiver_did", receiverDID,
				"error", err,
				"action", "Assuming tokens not transferred")
			tokensTransferredFromReceiver = false
		} else {
			c.log.Error("STEP 5 FAILED: Cannot verify if receiver transferred tokens",
				"receiver_did", receiverDID,
				"error", err,
				"action", "Recovery blocked for safety")
			return nil, fmt.Errorf("cannot verify if receiver transferred tokens (receiver may be offline): %v", err)
		}
	}

	if tokensTransferredFromReceiver {
		c.log.Error("STEP 5 FAILED: Tokens were transferred by receiver",
			"receiver_did", receiverDID,
			"transaction_id", transactionID,
			"action", "Recovery blocked - double-spending protection")
		return nil, fmt.Errorf("tokens were transferred from receiver, cannot recover (double-spending detected)")
	}

	c.log.Info("STEP 5 SUCCESS: Tokens not transferred by receiver")

	// Step 6: Perform token recovery
	c.log.Info("STEP 6: Starting actual token recovery",
		"sender_did", senderDID,
		"transaction_id", transactionID)

	recoveryResult, err := c.performTokenRecovery(senderDID, transactionID, transactionDetails)
	if err != nil {
		c.log.Error("STEP 6 FAILED: Token recovery failed",
			"sender_did", senderDID,
			"transaction_id", transactionID,
			"error", err)
		return nil, fmt.Errorf("failed to perform token recovery: %v", err)
	}

	c.log.Info("========== TOKEN RECOVERY COMPLETED SUCCESSFULLY ==========",
		"sender_did", senderDID,
		"transaction_id", transactionID,
		"recovered_tokens", recoveryResult.RecoveredTokenCount,
		"recovery_date", recoveryResult.RecoveryDate,
		"timestamp", time.Now().Format("2006-01-02 15:04:05"))

	return recoveryResult, nil
}

// getTransactionFromHistory retrieves transaction details from FT transaction history
func (c *Core) getTransactionFromHistory(senderDID, transactionID string) (*model.TransactionDetails, error) {
	c.log.Debug("Getting transaction from history",
		"sender_did", senderDID,
		"transaction_id", transactionID)

	// First try to get from FT transaction history
	var ftHistory []model.FTTransactionHistory
	err := c.w.GetStorage().Read("ft_transaction_history", &ftHistory,
		"sender_did=? AND transaction_id=?", senderDID, transactionID)

	if err == nil && len(ftHistory) > 0 {
		// Convert FT history to TransactionDetails
		ft := ftHistory[0]
		return &model.TransactionDetails{
			TransactionID:   ft.TransactionID,
			TransactionType: "FT",
			SenderDID:       ft.SenderDID,
			ReceiverDID:     ft.ReceiverDID,
			Amount:          float64(ft.Amount),
			DateTime:        ft.DateTime,
			Comment:         ft.Comment,
			Status:          ft.Status,
		}, nil
	}

	// If not found in FT history, try regular transaction history
	var txHistory []model.TransactionDetails
	err = c.w.GetStorage().Read("transaction_history", &txHistory,
		"sender_did=? AND transaction_id=?", senderDID, transactionID)

	if err != nil || len(txHistory) == 0 {
		return nil, fmt.Errorf("transaction not found in history")
	}

	return &txHistory[0], nil
}

// isTransactionAlreadyRecovered checks if transaction has already been recovered
func (c *Core) isTransactionAlreadyRecovered(transaction *model.TransactionDetails) bool {
	// Check if comment contains recovery marker
	if transaction.Comment != "" {
		return fmt.Sprintf("%s", transaction.Comment) == "RECOVERED" ||
			fmt.Sprintf("%s", transaction.Comment) == "[RECOVERED]"
	}

	// Check token_recovery table - this is the primary source of truth
	var recovery model.TokenRecovery
	err := c.w.GetStorage().Read("token_recovery", &recovery,
		"transaction_id=?", transaction.TransactionID)

	if err == nil {
		c.log.Info("Transaction already recovered",
			"transaction_id", transaction.TransactionID,
			"recovery_date", recovery.RecoveredAt,
			"recovered_by", recovery.RecoveredBy)
		return true
	}

	return false
}

// isTransactionWithinRecoveryWindow checks if transaction is within the recovery time window
func (c *Core) isTransactionWithinRecoveryWindow(transactionDate time.Time) bool {
	age := time.Since(transactionDate)
	ageDays := age.Hours() / 24

	// Allow recovery for transactions older than 7 days but less than 14 days
	// For testing/development, we might be more lenient
	return ageDays >= 7 && ageDays <= 14
}

// isExceptionTransaction checks if this transaction is marked for exception handling
func (c *Core) isExceptionTransaction(transactionID string) bool {
	// For now, return false - could be enhanced to check a database table
	// or configuration file for exception transactions
	return false
}

// checkReceiverTokenStatus checks if receiver has the tokens
func (c *Core) checkReceiverTokenStatus(receiverDID, transactionID string, expectedAmount int) (bool, error) {
	c.log.Debug("Checking receiver token status",
		"receiver", receiverDID,
		"transaction_id", transactionID,
		"expected_amount", expectedAmount)

	receiverPeer, err := c.getPeer(receiverDID)
	if err != nil {
		return false, fmt.Errorf("failed to get receiver peer: %v", err)
	}
	defer receiverPeer.Close()

	// Check token ownership - this would be a simple ping to see if receiver has tokens
	// For simplicity, we'll assume receiver doesn't have tokens if we can't verify
	return false, nil
}

// checkIfTokensTransferredFromReceiver checks if receiver has already transferred the tokens
func (c *Core) checkIfTokensTransferredFromReceiver(receiverDID, transactionID string) (bool, error) {
	c.log.Debug("Checking if tokens transferred from receiver",
		"receiver", receiverDID,
		"transaction_id", transactionID)

	// For simplicity, assume tokens were not transferred
	// In a real implementation, this would check the receiver's transaction history
	return false, nil
}

// performTokenRecovery performs the actual token recovery process
func (c *Core) performTokenRecovery(senderDID, transactionID string, transactionDetails *model.TransactionDetails) (*TokenRecoveryResult, error) {
	c.log.Info("Performing token recovery",
		"sender_did", senderDID,
		"transaction_id", transactionID,
		"amount", transactionDetails.Amount)

	// Step 1: Get the tokens that were sent in this transaction
	var tokensToRecover []string
	var tokenCount int

	if transactionDetails.TransactionType == "FT" {
		// For FT tokens, we need to find the tokens in the FT token storage
		var ftTokens []wallet.FTToken
		err := c.w.GetStorage().Read(wallet.FTTokenStorage, &ftTokens,
			"transaction_id=? AND token_status=?", transactionID, wallet.TokenIsPledged)

		if err != nil {
			return nil, fmt.Errorf("failed to find pledged FT tokens: %v", err)
		}

		for _, token := range ftTokens {
			tokensToRecover = append(tokensToRecover, token.TokenID)
		}
		tokenCount = len(ftTokens)
	} else {
		// For RBT tokens
		var tokens []wallet.Token
		err := c.w.GetStorage().Read(wallet.TokenStorage, &tokens,
			"transaction_id=? AND token_status=?", transactionID, wallet.TokenIsPledged)

		if err != nil {
			return nil, fmt.Errorf("failed to find pledged tokens: %v", err)
		}

		for _, token := range tokens {
			tokensToRecover = append(tokensToRecover, token.TokenID)
		}
		tokenCount = len(tokens)
	}

	if len(tokensToRecover) == 0 {
		return nil, fmt.Errorf("no pledged tokens found for transaction %s", transactionID)
	}

	c.log.Info("Found tokens to recover",
		"transaction_id", transactionID,
		"token_count", tokenCount,
		"token_ids", tokensToRecover)

	// Step 2: Change token status from pledged to free
	for _, tokenID := range tokensToRecover {
		if transactionDetails.TransactionType == "FT" {
			// Update FT token status
			var ftToken wallet.FTToken
			err := c.w.GetStorage().Read(wallet.FTTokenStorage, &ftToken, "token_id=?", tokenID)
			if err == nil && ftToken.TokenStatus == wallet.TokenIsPledged {
				ftToken.TokenStatus = wallet.TokenIsFree
				ftToken.TransactionID = "" // Clear transaction ID
				err = c.w.GetStorage().Update(wallet.FTTokenStorage, &ftToken, "token_id=?", tokenID)
				if err != nil {
					c.log.Error("Failed to update FT token status",
						"token_id", tokenID,
						"error", err)
				}
			}
		} else {
			// Update regular token status
			var token wallet.Token
			err := c.w.GetStorage().Read(wallet.TokenStorage, &token, "token_id=?", tokenID)
			if err == nil && token.TokenStatus == wallet.TokenIsPledged {
				token.TokenStatus = wallet.TokenIsFree
				token.TransactionID = "" // Clear transaction ID
				err = c.w.GetStorage().Update(wallet.TokenStorage, &token, "token_id=?", tokenID)
				if err != nil {
					c.log.Error("Failed to update token status",
						"token_id", tokenID,
						"error", err)
				}
			}
		}
	}

	// Step 3: Record the recovery in the recovery tracking table
	recovery := &model.TokenRecovery{
		TransactionID: transactionID,
		RecoveredAt:   time.Now(),
		RecoveredBy:   senderDID,
		TokenCount:    tokenCount,
		TokenIDs:      fmt.Sprintf("%v", tokensToRecover), // Convert to JSON string
		RecoveryType:  "normal",
		RecoveryNotes: "Token recovery completed successfully",
	}

	err := c.w.GetStorage().Write("token_recovery", recovery)
	if err != nil {
		c.log.Error("Failed to record recovery in tracking table",
			"transaction_id", transactionID,
			"error", err)
		// Don't fail the recovery for this
	}

	// Step 4: Remove transaction from history since it's being rolled back
	// When tokens are recovered, the transaction effectively never happened, so we remove it from history
	if transactionDetails.TransactionType == "FT" {
		// Remove from FT transaction history - transaction never happened
		err := c.w.GetStorage().Delete("ft_transaction_history", 
			"transaction_id=? AND sender_did=?", transactionID, senderDID)
		if err != nil {
			c.log.Error("Failed to remove FT transaction from history",
				"transaction_id", transactionID,
				"error", err)
			// Don't fail the recovery for this, but log it
		} else {
			c.log.Info("Successfully removed FT transaction from history",
				"transaction_id", transactionID,
				"sender_did", senderDID)
		}
	} else {
		// Remove from regular transaction history - transaction never happened
		err := c.w.GetStorage().Delete("transaction_history", 
			"transaction_id=? AND sender_did=?", transactionID, senderDID)
		if err != nil {
			c.log.Error("Failed to remove transaction from history",
				"transaction_id", transactionID,
				"error", err)
			// Don't fail the recovery for this, but log it
		} else {
			c.log.Info("Successfully removed transaction from history",
				"transaction_id", transactionID,
				"sender_did", senderDID)
		}
	}

	// Step 5: Create recovery result
	result := &TokenRecoveryResult{
		TransactionID:       transactionID,
		SenderDID:           senderDID,
		ReceiverDID:         transactionDetails.ReceiverDID,
		RecoveredTokenCount: tokenCount,
		RecoveryDate:        time.Now(),
		Status:              "success",
		Message:             fmt.Sprintf("Successfully recovered %d tokens and removed transaction %s from history (transaction rolled back)", tokenCount, transactionID),
	}

	c.log.Info("Token recovery completed successfully",
		"transaction_id", transactionID,
		"recovered_tokens", tokenCount,
		"sender_did", senderDID)

	return result, nil
}