package output

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestPrintJSON(t *testing.T) {
	out := captureStdout(func() {
		PrintJSON(map[string]string{"name": "test", "value": "123"})
	})
	assert.Contains(t, out, `"name": "test"`)
	assert.Contains(t, out, `"value": "123"`)
}

func TestPrintTable(t *testing.T) {
	out := captureStdout(func() {
		PrintTable(
			[]string{"NAME", "VALUE"},
			[][]string{
				{"foo", "bar"},
				{"hello", "world"},
			},
		)
	})
	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "VALUE")
	assert.Contains(t, out, "foo")
	assert.Contains(t, out, "bar")
	assert.Contains(t, out, "hello")
	assert.Contains(t, out, "world")
}

func TestPrintTable_Empty(t *testing.T) {
	out := captureStdout(func() {
		PrintTable([]string{"NAME"}, [][]string{})
	})
	assert.Contains(t, out, "No results found.")
}

func TestPrintKeyValue(t *testing.T) {
	out := captureStdout(func() {
		PrintKeyValue([][2]string{
			{"Name", "John"},
			{"Email", "john@example.com"},
		})
	})
	assert.Contains(t, out, "Name:")
	assert.Contains(t, out, "John")
	assert.Contains(t, out, "Email:")
	assert.Contains(t, out, "john@example.com")
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"abcdefghijkl", "••••••••ijkl"},
		{"abcd", "••••"},
		{"ab", "••"},
		{"", ""},
		{"a", "•"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := MaskSecret(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPrintJQ(t *testing.T) {
	data := []map[string]string{
		{"name": "foo"},
		{"name": "bar"},
	}
	out := captureStdout(func() {
		err := PrintJQ(data, ".[].name")
		assert.NoError(t, err)
	})
	assert.Contains(t, out, "foo")
	assert.Contains(t, out, "bar")
}

func TestPrintJQ_InvalidFilter(t *testing.T) {
	err := PrintJQ(map[string]string{}, ".[invalid")
	assert.Error(t, err)
}

func TestPrintSuccess(t *testing.T) {
	out := captureStdout(func() {
		PrintSuccess("Operation complete")
	})
	assert.Contains(t, out, "✓")
	assert.Contains(t, out, "Operation complete")
}
