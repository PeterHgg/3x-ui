package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mhsanaei/3x-ui/v2/database/model"
)

// ComputeClientsHash computes SHA256 hash of all client configurations
// Sorted by email to ensure consistent hashing
func ComputeClientsHash(clients []model.Client) (string, error) {
	if len(clients) == 0 {
		return "", nil
	}

	// Sort clients by email for consistent ordering
	sortedClients := make([]model.Client, len(clients))
	copy(sortedClients, clients)
	sort.Slice(sortedClients, func(i, j int) bool {
		return sortedClients[i].Email < sortedClients[j].Email
	})

	// Serialize to JSON
	data, err := json.Marshal(sortedClients)
	if err != nil {
		return "", fmt.Errorf("failed to marshal clients: %w", err)
	}

	// Compute SHA256
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}
