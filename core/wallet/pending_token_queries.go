package wallet

import (
	"time"
)

// GetPendingFTTokensOlderThan returns FT tokens that have been pending for longer than the specified duration
// Returns a map of transaction ID to token IDs
func (w *Wallet) GetPendingFTTokensOlderThan(duration time.Duration) (map[string][]string, error) {
	w.l.Lock()
	defer w.l.Unlock()
	
	// Query for pending FT tokens
	// Note: FTToken doesn't have updated_at column, so we can't filter by time
	// For now, return all pending tokens regardless of age
	// TODO: Add timestamp tracking to FTToken table for better filtering
	var pendingTokens []FTToken
	err := w.s.Read(FTTokenStorage, &pendingTokens, 
		"token_status = ?", 
		TokenIsPending)
	
	if err != nil {
		// No pending tokens found is not an error
		if err.Error() == "no records found" {
			return make(map[string][]string), nil
		}
		return nil, err
	}
	
	// Group by transaction ID
	result := make(map[string][]string)
	for _, token := range pendingTokens {
		if token.TransactionID != "" {
			result[token.TransactionID] = append(result[token.TransactionID], token.TokenID)
		}
	}
	
	return result, nil
}

// GetPendingTokensOlderThan returns RBT tokens that have been pending for longer than the specified duration
// Returns a map of transaction ID to token IDs
func (w *Wallet) GetPendingTokensOlderThan(duration time.Duration) (map[string][]string, error) {
	w.l.Lock()
	defer w.l.Unlock()
	
	// Query for pending tokens
	// Note: Token doesn't have updated_at column, so we can't filter by time
	// For now, return all pending tokens regardless of age
	// TODO: Add timestamp tracking to Token table for better filtering
	var pendingTokens []Token
	err := w.s.Read(TokenStorage, &pendingTokens, 
		"token_status = ?", 
		TokenIsPending)
	
	if err != nil {
		// No pending tokens found is not an error
		if err.Error() == "no records found" {
			return make(map[string][]string), nil
		}
		return nil, err
	}
	
	// Group by transaction ID
	result := make(map[string][]string)
	for _, token := range pendingTokens {
		if token.TransactionID != "" {
			result[token.TransactionID] = append(result[token.TransactionID], token.TokenID)
		}
	}
	
	return result, nil
}