package models

type PriceOption struct {
	Price    Money
	Deeplink string
}

type RuleResult struct {
	Type    RuleType
	Options []PriceOption
}

type PricesForRulesRequest struct {
	Origin      IATACode
	Destination IATACode
	TopN        uint32
	Rules       []Rule
}

type PricesForRulesResponse struct {
	Results []RuleResult
}
