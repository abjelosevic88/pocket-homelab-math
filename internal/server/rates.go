package server

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/abjelosevic88/pocket-homelab-math/internal/store"
)

// Exchange rates, per 1 euro. The European Central Bank publishes them every
// working day; they are fetched by the server in the background (never while
// someone waits) and kept in the cache table. Currencies pegged to the euro
// are always known, even with no internet.
const (
	ratesKey = "rates:ecb"
	ecbURL   = "https://www.ecb.europa.eu/stats/eurofxref/eurofxref-daily.xml"
)

var pegged = map[string]float64{"EUR": 1, "BAM": 1.95583, "BGN": 1.95583, "XOF": 655.957, "XAF": 655.957, "KMF": 491.96775, "CVE": 110.265, "DKK": 7.46038}

type ratesCache struct {
	Date    string             `json:"date"`
	Fetched string             `json:"fetched"`
	Rates   map[string]float64 `json:"rates"`
}

func (s *Server) cachedRates(ctx context.Context) (ratesCache, bool) {
	var rc ratesCache
	v, _, ok := s.store.CacheGet(ctx, ratesKey)
	if !ok || json.Unmarshal([]byte(v), &rc) != nil {
		return rc, false
	}
	return rc, true
}

// ratesFor is what a person's sums use: the pegged currencies, plus the
// bank's rates unless they switched them off.
func (s *Server) ratesFor(ctx context.Context, st store.Settings) map[string]float64 {
	out := map[string]float64{}
	if st.Rates != "off" {
		if rc, ok := s.cachedRates(ctx); ok {
			for k, v := range rc.Rates {
				if v > 0 {
					out[k] = v
				}
			}
		}
	}
	for k, v := range pegged {
		out[k] = v
	}
	return out
}

func (s *Server) ratesInfo(ctx context.Context) map[string]any {
	rc, ok := s.cachedRates(ctx)
	if !ok {
		return map[string]any{"source": "", "date": "", "note": "Only currencies tied to the euro (BAM, BGN, DKK…) until the server reaches the European Central Bank."}
	}
	return map[string]any{"source": "European Central Bank", "date": rc.Date, "fetched": rc.Fetched, "count": len(rc.Rates) + 1}
}

// refreshRates fetches the day's rates when the kept ones are older than
// twelve hours.
func (s *Server) refreshRates(ctx context.Context, force bool) error {
	if rc, ok := s.cachedRates(ctx); ok && !force {
		if t, err := time.Parse(time.RFC3339, rc.Fetched); err == nil && time.Since(t) < 12*time.Hour {
			return nil
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ecbURL, nil)
	req.Header.Set("User-Agent", s.cfg.UserAgent)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("ECB answered %s", res.Status)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	rc, err := parseECB(b)
	if err != nil {
		return err
	}
	rc.Fetched = time.Now().UTC().Format(time.RFC3339)
	j, _ := json.Marshal(rc)
	return s.store.CachePut(ctx, ratesKey, string(j))
}

func parseECB(b []byte) (ratesCache, error) {
	var doc struct {
		Cube struct {
			Cube []struct {
				Time string `xml:"time,attr"`
				Cube []struct {
					Currency string `xml:"currency,attr"`
					Rate     string `xml:"rate,attr"`
				} `xml:"Cube"`
			} `xml:"Cube"`
		} `xml:"Cube"`
	}
	if err := xml.Unmarshal(b, &doc); err != nil {
		return ratesCache{}, err
	}
	rc := ratesCache{Rates: map[string]float64{}}
	for _, day := range doc.Cube.Cube {
		rc.Date = day.Time
		for _, c := range day.Cube {
			if v, err := strconv.ParseFloat(c.Rate, 64); err == nil && v > 0 && len(c.Currency) == 3 {
				rc.Rates[c.Currency] = v
			}
		}
		break
	}
	if len(rc.Rates) == 0 {
		return rc, fmt.Errorf("no rates in the answer")
	}
	return rc, nil
}

func (s *Server) handleRates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method == http.MethodPost {
		if err := s.refreshRates(ctx, true); err != nil {
			writeErr(w, http.StatusBadGateway, "the rates could not be fetched: "+err.Error())
			return
		}
	}
	st := s.settingsOf(ctx, actorID(ctx))
	writeJSON(w, http.StatusOK, map[string]any{"base": "EUR", "rates": s.ratesFor(ctx, st), "info": s.ratesInfo(ctx), "currency": st.Currency})
}

func sortedStrings(v []string) []string {
	sort.Strings(v)
	return v
}
