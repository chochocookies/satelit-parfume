package products

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ImportRow is one parsed CSV row, matching the column order from the
// spec's CSV import schema (section 48): name, sku, barcode, brand,
// category, price, description, size, gender, image_url — the same
// header seeds/products_verified.csv already uses.
type ImportRow struct {
	RowNumber   int
	Name        string
	SKU         string
	Barcode     string
	Brand       string
	Category    string
	Price       string
	Description string
	Size        string
	Gender      string
	ImageURL    string
}

type RowError struct {
	Row    int    `json:"row"`
	Reason string `json:"reason"`
}

type ImportResult struct {
	Imported int        `json:"imported"`
	Skipped  int        `json:"skipped"`
	Errors   []RowError `json:"errors"`
}

var expectedHeader = []string{
	"name", "sku", "barcode", "brand", "category", "price", "description", "size", "gender", "image_url",
}

// ParseCSV does pure structural parsing — header shape + column count —
// with no database access, so it's unit-testable on its own (see
// import_test.go). Business-rule validation (required fields, numeric
// price) happens separately in validateRow; duplicate detection needs the
// database and happens in Repository.Import.
func ParseCSV(r io.Reader) ([]ImportRow, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = len(expectedHeader)

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	if err := validateHeader(header); err != nil {
		return nil, err
	}

	var rows []ImportRow
	rowNum := 1 // row 1 = first data row, right after the header
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row %d: %w", rowNum, err)
		}

		rows = append(rows, ImportRow{
			RowNumber:   rowNum,
			Name:        strings.TrimSpace(record[0]),
			SKU:         strings.TrimSpace(record[1]),
			Barcode:     strings.TrimSpace(record[2]),
			Brand:       strings.TrimSpace(record[3]),
			Category:    strings.TrimSpace(record[4]),
			Price:       strings.TrimSpace(record[5]),
			Description: strings.TrimSpace(record[6]),
			Size:        strings.TrimSpace(record[7]),
			Gender:      strings.TrimSpace(record[8]),
			ImageURL:    strings.TrimSpace(record[9]),
		})
		rowNum++
	}

	return rows, nil
}

func validateHeader(header []string) error {
	if len(header) != len(expectedHeader) {
		return fmt.Errorf("expected %d columns (%s), got %d",
			len(expectedHeader), strings.Join(expectedHeader, ","), len(header))
	}
	for i, want := range expectedHeader {
		if strings.EqualFold(strings.TrimSpace(header[i]), want) {
			continue
		}
		return fmt.Errorf("column %d: expected %q, got %q", i+1, want, header[i])
	}
	return nil
}

// validateRow checks one row's business rules independent of the
// database — required fields, a numeric integer-Rupiah price (section
// 85: never floating point). Kept pure/deterministic so it's testable
// without Postgres; duplicate-name/SKU detection needs the database and
// lives in Repository.importRow instead.
func validateRow(row ImportRow) (priceRupiah int64, problems []string) {
	if row.Name == "" {
		problems = append(problems, "name is required")
	}

	if row.Price == "" {
		problems = append(problems, "price is required")
	} else {
		p, err := strconv.ParseInt(row.Price, 10, 64)
		if err != nil || p < 0 {
			problems = append(problems, "price must be a whole number of Rupiah, e.g. 35000 (not 35000.00 or Rp35.000)")
		} else {
			priceRupiah = p
		}
	}

	return priceRupiah, problems
}
