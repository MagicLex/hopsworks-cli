package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"golang.org/x/term"
)

var JSONMode bool

// ANSI codes — only used when stdout is a terminal
var (
	bold  = "\033[1m"
	dim   = "\033[2m"
	reset = "\033[0m"
	green = "\033[32m"
	red   = "\033[31m"
)

func init() {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		bold, dim, reset, green, red = "", "", "", "", ""
	}
}

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

	if len(rows) == 0 {
		fmt.Printf("%s(none)%s\n", dim, reset)
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Bold headers
	boldHeaders := make([]string, len(headers))
	for i, h := range headers {
		boldHeaders[i] = bold + h + reset
	}
	fmt.Fprintln(w, strings.Join(boldHeaders, "\t"))

	// Separator
	sepParts := make([]string, len(headers))
	for i, h := range headers {
		sepParts[i] = dim + strings.Repeat("-", len(h)) + reset
	}
	fmt.Fprintln(w, strings.Join(sepParts, "\t"))

	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	w.Flush()
}

// Success prints a success message (suppressed in JSON mode)
func Success(format string, args ...interface{}) {
	if !JSONMode {
		fmt.Printf(green+"  "+reset+format+"\n", args...)
	}
}

// Info prints an info message (suppressed in JSON mode)
func Info(format string, args ...interface{}) {
	if !JSONMode {
		fmt.Printf(dim+"  "+reset+format+"\n", args...)
	}
}

// Error prints an error to stderr
func Error(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, red+"  "+reset+format+"\n", args...)
}
