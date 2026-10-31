package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"cloud-pos/models"
)

func TestOfflineClassificationReportsWithoutDatabase(t *testing.T) {
	input := `[{"product_id":"one","name":"Nasi"},{"product_id":"two","name":"=Kopi"},{"product_id":"three","name":"Tidak dikenal"}]`
	var output bytes.Buffer
	if err := run(strings.NewReader(input), &output, "json"); err != nil {
		t.Fatal(err)
	}
	var report models.ProductClassificationReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil || report.Totals.Total != 3 || report.Totals.NeedsReview != 1 {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	output.Reset()
	if err := run(strings.NewReader(input), &output, "csv"); err != nil || !strings.Contains(output.String(), "'=Kopi") {
		t.Fatalf("CSV formula protection: %s %v", output.String(), err)
	}
	for _, invalid := range []string{`{}[]`, `[] {}`, `[{`} {
		if err := run(strings.NewReader(invalid), &bytes.Buffer{}, "json"); err == nil {
			t.Fatalf("invalid input accepted: %s", invalid)
		}
	}
}
