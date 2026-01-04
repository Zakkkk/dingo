package lsp

import (
	"encoding/json"
	"fmt"

	"go.lsp.dev/protocol"
)

// CallHierarchyContext tracks original Go items for the stateful call hierarchy protocol.
//
// The call hierarchy protocol is stateful:
// 1. prepare returns CallHierarchyItem (we translate to Dingo positions for the client)
// 2. Client sends back item for incomingCalls/outgoingCalls
// 3. gopls expects original Go item, not translated Dingo item
//
// Solution: Use the Data field (opaque JSON) to embed the original Go item.
// This avoids the need for a mutex-protected map and handles the case where
// the client may modify non-essential fields.
type CallHierarchyContext struct {
	// No state needed - we use the Data field approach
}

// NewCallHierarchyContext creates a new context for call hierarchy state tracking
func NewCallHierarchyContext() *CallHierarchyContext {
	return &CallHierarchyContext{}
}

// embeddedGoItem is the structure embedded in the Data field
type embeddedGoItem struct {
	// OriginalGoItem contains the serialized original Go item from gopls
	OriginalGoItem protocol.CallHierarchyItem `json:"originalGoItem"`
}

// EmbedGoItem serializes the original Go item into the Data field of a Dingo item.
// This allows us to recover the original Go item when the client sends it back.
//
// The dingoItem is modified in place (Data field is set).
func (c *CallHierarchyContext) EmbedGoItem(dingoItem *protocol.CallHierarchyItem, goItem protocol.CallHierarchyItem) error {
	// Serialize the original Go item
	embedded := embeddedGoItem{
		OriginalGoItem: goItem,
	}

	data, err := json.Marshal(embedded)
	if err != nil {
		return fmt.Errorf("failed to marshal Go item for Data field: %w", err)
	}

	// Store in the Data field (opaque JSON that the client should preserve)
	dingoItem.Data = data
	return nil
}

// RecoverGoItem extracts the original Go item from the Data field.
// Returns the original Go item that gopls expects.
//
// The Data field is interface{} and may come back in different forms:
// - json.RawMessage ([]byte) if parsed directly
// - map[string]interface{} if re-parsed by the client
func (c *CallHierarchyContext) RecoverGoItem(dingoItem protocol.CallHierarchyItem) (protocol.CallHierarchyItem, error) {
	if dingoItem.Data == nil {
		return protocol.CallHierarchyItem{}, fmt.Errorf("no embedded Go item in Data field")
	}

	// Convert Data to JSON bytes for unmarshaling
	var dataBytes []byte
	switch data := dingoItem.Data.(type) {
	case []byte:
		// Already bytes (e.g., json.RawMessage)
		dataBytes = data
	case string:
		// String representation
		dataBytes = []byte(data)
	default:
		// Need to re-marshal (e.g., map[string]interface{} from client)
		var err error
		dataBytes, err = json.Marshal(data)
		if err != nil {
			return protocol.CallHierarchyItem{}, fmt.Errorf("failed to marshal Data field: %w", err)
		}
	}

	if len(dataBytes) == 0 {
		return protocol.CallHierarchyItem{}, fmt.Errorf("empty Data field")
	}

	var embedded embeddedGoItem
	if err := json.Unmarshal(dataBytes, &embedded); err != nil {
		return protocol.CallHierarchyItem{}, fmt.Errorf("failed to unmarshal embedded Go item: %w", err)
	}

	return embedded.OriginalGoItem, nil
}

// TranslateItemToDingo translates a gopls CallHierarchyItem to Dingo positions
// and embeds the original Go item in the Data field for recovery.
func (c *CallHierarchyContext) TranslateItemToDingo(
	translator *Translator,
	goItem protocol.CallHierarchyItem,
) (protocol.CallHierarchyItem, error) {
	// First translate the item to Dingo positions
	dingoItem, err := translator.TranslateCallHierarchyItem(goItem, GoToDingo)
	if err != nil {
		return goItem, err
	}

	// Embed the original Go item in the Data field
	if err := c.EmbedGoItem(&dingoItem, goItem); err != nil {
		return dingoItem, err
	}

	return dingoItem, nil
}

// TranslateItemsToDingo translates a slice of gopls CallHierarchyItems to Dingo
// positions and embeds original Go items in their Data fields.
func (c *CallHierarchyContext) TranslateItemsToDingo(
	translator *Translator,
	goItems []protocol.CallHierarchyItem,
) ([]protocol.CallHierarchyItem, error) {
	if len(goItems) == 0 {
		return goItems, nil
	}

	dingoItems := make([]protocol.CallHierarchyItem, 0, len(goItems))
	for _, goItem := range goItems {
		dingoItem, err := c.TranslateItemToDingo(translator, goItem)
		if err != nil {
			// Skip items that can't be translated
			continue
		}
		dingoItems = append(dingoItems, dingoItem)
	}

	return dingoItems, nil
}
