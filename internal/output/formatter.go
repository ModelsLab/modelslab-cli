package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/itchyny/gojq"
	"github.com/tidwall/gjson"
)

type Format string

const (
	FormatHuman      Format = "human"
	FormatJSON       Format = "json"
	FormatStreamJSON Format = "stream-json"
)

// PrintJSON outputs data as formatted JSON.
func PrintJSON(data interface{}) {
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting JSON: %v\n", err)
		return
	}
	fmt.Println(string(out))
}

// PrintJSONCompact outputs data as compact JSON.
func PrintJSONCompact(data interface{}) {
	out, err := json.Marshal(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting JSON: %v\n", err)
		return
	}
	fmt.Println(string(out))
}

// PrintJQ applies a jq filter to JSON data and prints results.
func PrintJQ(data interface{}, filter string) error {
	query, err := gojq.Parse(filter)
	if err != nil {
		return fmt.Errorf("invalid jq filter: %w", err)
	}

	code, err := gojq.Compile(query)
	if err != nil {
		return fmt.Errorf("could not compile jq filter: %w", err)
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	var input interface{}
	json.Unmarshal(jsonBytes, &input)

	iter := code.Run(input)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			return err
		}
		out, _ := json.MarshalIndent(v, "", "  ")
		fmt.Println(string(out))
	}
	return nil
}

// GJSONFilter applies a GJSON path to raw JSON bytes.
func GJSONFilter(jsonBytes []byte, path string) string {
	result := gjson.GetBytes(jsonBytes, path)
	return result.String()
}

// PrintTable prints data as an aligned table.
func PrintTable(headers []string, rows [][]string) {
	if len(rows) == 0 {
		fmt.Println("No results found.")
		return
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print header
	headerLine := ""
	for i, h := range headers {
		if i > 0 {
			headerLine += "  "
		}
		headerLine += fmt.Sprintf("%-*s", widths[i], h)
	}
	fmt.Println(headerLine)

	// Print rows
	for _, row := range rows {
		line := ""
		for i, cell := range row {
			if i > 0 {
				line += "  "
			}
			if i < len(widths) {
				line += fmt.Sprintf("%-*s", widths[i], cell)
			} else {
				line += cell
			}
		}
		fmt.Println(line)
	}
}

// PrintKeyValue prints key-value pairs aligned.
func PrintKeyValue(pairs [][2]string) {
	maxKeyLen := 0
	for _, p := range pairs {
		if len(p[0]) > maxKeyLen {
			maxKeyLen = len(p[0])
		}
	}
	for _, p := range pairs {
		fmt.Printf("%-*s  %s\n", maxKeyLen, p[0]+":", p[1])
	}
}

// MaskSecret masks a secret string, showing only the last 4 chars.
func MaskSecret(s string) string {
	if len(s) <= 4 {
		return strings.Repeat("•", len(s))
	}
	return strings.Repeat("•", 8) + s[len(s)-4:]
}

// PrintError prints a formatted error message with suggestions.
func PrintError(msg string, suggestions ...string) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	if len(suggestions) > 0 {
		fmt.Fprintln(os.Stderr)
		for _, s := range suggestions {
			fmt.Fprintf(os.Stderr, "  %s\n", s)
		}
	}
}

// PrintSuccess prints a success message.
func PrintSuccess(msg string) {
	fmt.Printf("✓ %s\n", msg)
}
