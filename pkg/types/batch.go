package types

// BatchItem represents an item in a batch operation
type BatchItem struct {
	Key       string                 `json:"key"`
	Operation string                 `json:"operation"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// BatchOperation represents a completed batch operation
type BatchOperation struct {
	Operation string `json:"operation"`
	Key       string `json:"key"`
	Success   bool   `json:"success"`
	Error     error  `json:"error,omitempty"`
}
