package calc

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

type vector struct {
	Lines []string `json:"lines"`
	Want  []string `json:"want"`
	Alt   []string `json:"alt"`
	Kind  []string `json:"kind"`
	Error []bool   `json:"error"`
	Deg   bool     `json:"deg"`
}

// TestRates are fixed rates the vectors use (the web app's tests use the same).
var TestRates = map[string]float64{"EUR": 1, "BAM": 1.95583, "USD": 1.1, "GBP": 0.85}

func TestVectors(t *testing.T) {
	b, err := os.ReadFile("testdata/vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var vs []vector
	if err := json.Unmarshal(b, &vs); err != nil {
		t.Fatal(err)
	}
	today := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	for _, v := range vs {
		lines, _ := EvalSheet(v.Lines, Options{Units: true, Deg: v.Deg, Rates: TestRates, Today: today, Format: Format{Group: ","}})
		for i, ln := range lines {
			if i < len(v.Error) && v.Error[i] {
				if ln.Kind != "error" {
					t.Errorf("%q line %d: want an error, got %q", v.Lines, i, ln.Text)
				}
				continue
			}
			if ln.Kind == "error" {
				t.Errorf("%q line %d: %s", v.Lines, i, ln.Error)
				continue
			}
			if i < len(v.Want) && ln.Text != v.Want[i] {
				t.Errorf("%q line %d: got %q want %q", v.Lines, i, ln.Text, v.Want[i])
			}
			if i < len(v.Alt) && ln.Alt != v.Alt[i] {
				t.Errorf("%q line %d: alt %q want %q", v.Lines, i, ln.Alt, v.Alt[i])
			}
			if i < len(v.Kind) && ln.Kind != v.Kind[i] {
				t.Errorf("%q line %d: kind %q want %q", v.Lines, i, ln.Kind, v.Kind[i])
			}
		}
	}
}

func TestRun(t *testing.T) {
	two := 2
	res := Run("area = width * height\nlitres = area * coats / coverage",
		[]Input{{Name: "width", Unit: "m"}, {Name: "height", Unit: "m"}, {Name: "coats", Default: "2"}, {Name: "coverage", Unit: "m²/L", Default: "10"}},
		map[string]string{"width": "4", "height": "2.5"},
		[]Output{{Label: "Paint", Expr: "litres", Unit: "L", Decimals: &two, Main: true}}, Options{Units: true})
	if res.Error != "" || len(res.Outputs) != 1 || res.Outputs[0].Text != "2 L" {
		t.Fatalf("%+v", res)
	}
}
