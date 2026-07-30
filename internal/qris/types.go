package qris

type TLV struct {
	Tag      string
	Name     string
	Length   int
	Value    string
	Children []TLV
}

type Data struct {
	Version              string
	Method               string // "static" | "dynamic"
	MerchantAccountInfo  []MerchantAccountInfo
	MerchantCategoryCode string
	Currency             string
	Amount               string
	TipIndicator         string // "prompt" | "fixed" | "percentage"
	TipFixed             string
	TipPercentage        string
	CountryCode          string
	MerchantName         string
	MerchantCity         string
	PostalCode           string
	CRC                  string
	Raw                  []TLV
}

type MerchantAccountInfo struct {
	Tag              string
	GloballyUniqueID string
	MerchantID       string
	MerchantCriteria string
	Fields           []TLV
}

type ConvertOptions struct {
	Amount int
	Fee    *Fee
}

type Fee struct {
	Type  string // "fixed" | "percentage"
	Value float64
}

type ValidationResult struct {
	Valid  bool
	Errors []string
}
