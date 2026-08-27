package products

import (
	"strings"
	"testing"
)

func TestParseCSV(t *testing.T) {
	input := "name,sku,barcode,brand,category,price,description,size,gender,image_url\n" +
		"Aqua Kiss,,,,Perfume,35000,,,,\n" +
		"Mix Aroma Unisex Cowo-Cewe,,,,Mix Perfume,55000,,,,\n"

	rows, err := ParseCSV(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCSV() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("ParseCSV() returned %d rows, want 2", len(rows))
	}

	first := rows[0]
	if first.RowNumber != 1 {
		t.Errorf("rows[0].RowNumber = %d, want 1", first.RowNumber)
	}
	if first.Name != "Aqua Kiss" {
		t.Errorf("rows[0].Name = %q, want %q", first.Name, "Aqua Kiss")
	}
	if first.Category != "Perfume" {
		t.Errorf("rows[0].Category = %q, want %q", first.Category, "Perfume")
	}
	if first.Price != "35000" {
		t.Errorf("rows[0].Price = %q, want %q", first.Price, "35000")
	}
	if first.SKU != "" {
		t.Errorf("rows[0].SKU = %q, want empty (not fabricated)", first.SKU)
	}

	if rows[1].RowNumber != 2 {
		t.Errorf("rows[1].RowNumber = %d, want 2", rows[1].RowNumber)
	}
}

func TestParseCSVRejectsWrongHeader(t *testing.T) {
	input := "name,price\nAqua Kiss,35000\n"
	if _, err := ParseCSV(strings.NewReader(input)); err == nil {
		t.Error("ParseCSV() with a mismatched header should error, got nil")
	}
}

func TestParseCSVHeaderIsCaseInsensitive(t *testing.T) {
	input := "Name,SKU,Barcode,Brand,Category,Price,Description,Size,Gender,Image_URL\nAqua Kiss,,,,Perfume,35000,,,,\n"
	rows, err := ParseCSV(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCSV() with a differently-cased header should still work, got error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
}

func TestValidateRow(t *testing.T) {
	cases := []struct {
		name         string
		row          ImportRow
		wantPrice    int64
		wantProblems int
	}{
		{
			name:      "valid row",
			row:       ImportRow{Name: "Aqua Kiss", Price: "35000"},
			wantPrice: 35000,
		},
		{
			name:         "missing name",
			row:          ImportRow{Price: "35000"},
			wantProblems: 1,
		},
		{
			name:         "missing price",
			row:          ImportRow{Name: "Aqua Kiss"},
			wantProblems: 1,
		},
		{
			name:         "non-numeric price (Rp formatting wasn't stripped)",
			row:          ImportRow{Name: "Aqua Kiss", Price: "Rp35.000"},
			wantProblems: 1,
		},
		{
			name:         "floating point price is rejected — section 85: integer Rupiah only",
			row:          ImportRow{Name: "Aqua Kiss", Price: "35000.00"},
			wantProblems: 1,
		},
		{
			name:         "negative price",
			row:          ImportRow{Name: "Aqua Kiss", Price: "-100"},
			wantProblems: 1,
		},
		{
			name:         "missing both name and price",
			row:          ImportRow{},
			wantProblems: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			price, problems := validateRow(tc.row)
			if len(problems) != tc.wantProblems {
				t.Errorf("validateRow() problems = %v (len %d), want len %d", problems, len(problems), tc.wantProblems)
			}
			if tc.wantProblems == 0 && price != tc.wantPrice {
				t.Errorf("validateRow() price = %d, want %d", price, tc.wantPrice)
			}
		})
	}
}
