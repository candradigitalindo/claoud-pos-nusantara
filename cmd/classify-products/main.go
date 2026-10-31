// classify-products reads a JSON catalog from stdin and prints a review report.
// It has no database connection or write operation.
package main

import (
	"cloud-pos/internal/productclassification"
	"cloud-pos/models"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

func run(input io.Reader, output io.Writer, format string) error {
	if format != "json" && format != "csv" {
		return fmt.Errorf("format harus json atau csv")
	}
	var rows []models.ProductClassificationInput
	decoder := json.NewDecoder(input)
	if err := decoder.Decode(&rows); err != nil {
		return err
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("input harus berupa satu array JSON")
	}
	report := productclassification.Report(rows)
	if format == "json" {
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}
	w := csv.NewWriter(output)
	if err := w.Write([]string{"product_id", "outlet_id", "outlet_name", "name", "category_name", "current_kind", "suggested_kind", "status", "confidence", "reason", "fingerprint"}); err != nil {
		return err
	}
	for _, r := range report.Rows {
		values := []string{r.ProductID, r.OutletID, r.OutletName, r.Name, r.CategoryName, r.CurrentKind, r.SuggestedKind, r.Status, r.Confidence, r.Reason, r.Fingerprint}
		for i, value := range values {
			// Prevent exported catalog names becoming spreadsheet formulas.
			if len(value) > 0 && (value[0] == '=' || value[0] == '+' || value[0] == '-' || value[0] == '@' || value[0] == '\t' || value[0] == '\r') {
				values[i] = "'" + value
			}
		}
		if err := w.Write(values); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func main() {
	format := flag.String("format", "json", "report format: json or csv")
	flag.Parse()
	if err := run(os.Stdin, os.Stdout, *format); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
