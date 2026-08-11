package models

import "strings"

// NormalizePlanMonetization synthesizes prices[] / provider_products[] from legacy
// columns (and the reverse) so older seeds and new Console writes both work.
func NormalizePlanMonetization(p *Plan) {
	if p == nil {
		return
	}
	if len(p.Prices) == 0 && (p.Currency != "" || p.PriceMonthly > 0) {
		cur := strings.TrimSpace(p.Currency)
		if cur == "" {
			cur = "BDT"
		}
		p.Prices = []PlanPrice{{Currency: cur, Amount: p.PriceMonthly, Default: true}}
	}
	if len(p.ProviderProducts) == 0 {
		var rows []PlanProviderProduct
		if pid := strings.TrimSpace(p.PlayProductID); pid != "" {
			rows = append(rows, PlanProviderProduct{
				Provider:  "google_play",
				ProductID: pid,
				VariantID: strings.TrimSpace(p.PlayBasePlanID),
			})
		}
		if pid := strings.TrimSpace(p.PaddlePriceID); pid != "" {
			rows = append(rows, PlanProviderProduct{
				Provider:  "paddle",
				ProductID: pid,
			})
		}
		p.ProviderProducts = rows
	}
	SyncPlanLegacyMonetization(p)
}

// SyncPlanLegacyMonetization derives currency / price_monthly / play_* / paddle_*
// from prices[] and provider_products[] for older callers.
func SyncPlanLegacyMonetization(p *Plan) {
	if p == nil {
		return
	}
	if len(p.Prices) > 0 {
		def := p.Prices[0]
		for _, row := range p.Prices {
			if row.Default {
				def = row
				break
			}
		}
		if cur := strings.TrimSpace(def.Currency); cur != "" {
			p.Currency = cur
		}
		p.PriceMonthly = def.Amount
	}
	for _, row := range p.ProviderProducts {
		prov := strings.ToLower(strings.TrimSpace(row.Provider))
		pid := strings.TrimSpace(row.ProductID)
		if pid == "" {
			continue
		}
		switch prov {
		case "google_play", "play", "google":
			p.PlayProductID = pid
			if v := strings.TrimSpace(row.VariantID); v != "" {
				p.PlayBasePlanID = v
			}
		case "paddle":
			p.PaddlePriceID = pid
		}
	}
}

// DefaultPlanPrice returns the default (or first) price row.
func DefaultPlanPrice(p *Plan) (currency string, amount float64) {
	if p == nil {
		return "BDT", 0
	}
	NormalizePlanMonetization(p)
	if len(p.Prices) == 0 {
		cur := strings.TrimSpace(p.Currency)
		if cur == "" {
			cur = "BDT"
		}
		return cur, p.PriceMonthly
	}
	def := p.Prices[0]
	for _, row := range p.Prices {
		if row.Default {
			def = row
			break
		}
	}
	cur := strings.TrimSpace(def.Currency)
	if cur == "" {
		cur = "BDT"
	}
	return cur, def.Amount
}

// PlanPriceForCurrency finds a price row for currency (case-insensitive), else default.
func PlanPriceForCurrency(p *Plan, currency string) (string, float64) {
	NormalizePlanMonetization(p)
	want := strings.ToUpper(strings.TrimSpace(currency))
	if p != nil && want != "" {
		for _, row := range p.Prices {
			if strings.EqualFold(strings.TrimSpace(row.Currency), want) {
				cur := strings.TrimSpace(row.Currency)
				if cur == "" {
					cur = want
				}
				return cur, row.Amount
			}
		}
	}
	return DefaultPlanPrice(p)
}
