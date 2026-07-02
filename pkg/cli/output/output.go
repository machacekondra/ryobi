package output

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
)

// PrintJSON prints data as formatted JSON.
func PrintJSON(data any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// PrintTable prints a table with headers and rows.
func PrintTable(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Print headers
	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(w, "\t")
		}
		fmt.Fprint(w, h)
	}
	fmt.Fprintln(w)

	// Print rows
	for _, row := range rows {
		for i, col := range row {
			if i > 0 {
				fmt.Fprint(w, "\t")
			}
			fmt.Fprint(w, col)
		}
		fmt.Fprintln(w)
	}

	w.Flush()
}

// PrintSuccess prints a success message.
func PrintSuccess(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}

// PrintError prints an error message to stderr.
func PrintError(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}

// PrintStatus prints a status update.
func PrintStatus(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}
