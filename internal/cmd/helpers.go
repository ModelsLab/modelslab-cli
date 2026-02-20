package cmd

import "fmt"

// extractItems extracts the list of items from an API response.
// The API may return data as:
//   - result["data"].([]interface{}) — direct array
//   - result["data"].(map)["items"].([]interface{}) — paginated items
func extractItems(result map[string]interface{}) []interface{} {
	data, ok := result["data"]
	if !ok {
		return nil
	}

	// Direct array
	if arr, ok := data.([]interface{}); ok {
		return arr
	}

	// Nested items (paginated response)
	if m, ok := data.(map[string]interface{}); ok {
		if items, ok := m["items"].([]interface{}); ok {
			return items
		}
	}

	return nil
}

// extractData extracts the data object from an API response.
// Returns the data map, falling back to the result itself.
func extractData(result map[string]interface{}) map[string]interface{} {
	if data, ok := result["data"].(map[string]interface{}); ok {
		return data
	}
	return result
}

// firstNonNil returns the string value of the first non-nil key found in the map.
func firstNonNil(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			s := fmt.Sprintf("%v", v)
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}
