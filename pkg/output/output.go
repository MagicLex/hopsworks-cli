package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

var JSONMode bool

// PrintJSON outputs data as formatted JSON
func PrintJSON(v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting JSON: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}

// Table prints a formatted table with headers and rows
func Table(headers []string, rows [][]string) {
	if JSONMode {
		// Convert table to JSON array of objects
		var items []map[string]string
		for _, row := range rows {
			item := make(map[string]string)
			for i, h := range headers {
				if i < len(row) {
					item[strings.ToLower(h)] = row[i]
				}
			}
			items = append(items, item)
		}
		PrintJSON(items)
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	// Print headers
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	// Print rows
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	w.Flush()
}

// Success prints a success message (suppressed in JSON mode)
func Success(format string, args ...interface{}) {
	if !JSONMode {
		fmt.Printf(format+"\n", args...)
	}
}

// Info prints an info message (suppressed in JSON mode)
func Info(format string, args ...interface{}) {
	if !JSONMode {
		fmt.Printf(format+"\n", args...)
	}
}

// Error prints an error to stderr
func Error(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}
