package qris

import "strconv"

var tagNames = map[string]string{
	"00": "Payload Format Indicator",
	"01": "Point of Initiation Method",
	"52": "Merchant Category Code",
	"53": "Transaction Currency",
	"54": "Transaction Amount",
	"55": "Tip or Convenience Indicator",
	"56": "Value of Convenience Fee (Fixed)",
	"57": "Value of Convenience Fee (%)",
	"58": "Country Code",
	"59": "Merchant Name",
	"60": "Merchant City",
	"61": "Postal Code",
	"62": "Additional Data Field",
	"63": "CRC",
}

func isNestedTag(tag string) bool {
	if tag == "62" {
		return true
	}
	n, err := strconv.Atoi(tag)
	return err == nil && n >= 26 && n <= 51
}

func tagName(tag string) string {
	if n, ok := tagNames[tag]; ok {
		return n
	}
	if isNestedTag(tag) {
		return "Merchant Account Information"
	}
	return "Unknown (" + tag + ")"
}

// ParseTLV decodes a raw EMVCo TLV string.
func ParseTLV(data string) []TLV {
	var elements []TLV
	pos := 0
	for pos+4 <= len(data) {
		tag := data[pos : pos+2]
		length, err := strconv.Atoi(data[pos+2 : pos+4])
		if err != nil || pos+4+length > len(data) {
			break
		}
		value := data[pos+4 : pos+4+length]
		el := TLV{Tag: tag, Name: tagName(tag), Length: length, Value: value}
		if isNestedTag(tag) {
			el.Children = ParseTLV(value)
		}
		elements = append(elements, el)
		pos += 4 + length
	}
	return elements
}

// Parse decodes a QRIS string into structured data.
func Parse(qrisString string) Data {
	raw := ParseTLV(qrisString)
	find := func(tag string) *TLV {
		for i := range raw {
			if raw[i].Tag == tag {
				return &raw[i]
			}
		}
		return nil
	}

	method := "static"
	if find("01") != nil && find("01").Value == "12" {
		method = "dynamic"
	}

	var tipIndicator string
	if t := find("55"); t != nil {
		switch t.Value {
		case "01":
			tipIndicator = "prompt"
		case "02":
			tipIndicator = "fixed"
		case "03":
			tipIndicator = "percentage"
		}
	}

	var accounts []MerchantAccountInfo
	for _, t := range raw {
		n, err := strconv.Atoi(t.Tag)
		if err != nil || n < 26 || n > 51 || len(t.Children) == 0 {
			continue
		}
		findChild := func(ct string) string {
			for _, c := range t.Children {
				if c.Tag == ct {
					return c.Value
				}
			}
			return ""
		}
		mid := findChild("01")
		if mid == "" {
			mid = findChild("02")
		}
		accounts = append(accounts, MerchantAccountInfo{
			Tag:              t.Tag,
			GloballyUniqueID: findChild("00"),
			MerchantID:       mid,
			MerchantCriteria: findChild("03"),
			Fields:           t.Children,
		})
	}

	d := Data{
		Method:               method,
		MerchantAccountInfo:  accounts,
		MerchantCategoryCode: tlvValue(find("52")),
		Currency:             defaultStr(tlvValue(find("53")), "360"),
		CountryCode:          defaultStr(tlvValue(find("58")), "ID"),
		MerchantName:         tlvValue(find("59")),
		MerchantCity:         tlvValue(find("60")),
		PostalCode:           tlvValue(find("61")),
		Raw:                  raw,
	}
	if find("00") != nil {
		d.Version = find("00").Value
	} else {
		d.Version = "01"
	}
	if find("54") != nil {
		d.Amount = find("54").Value
	}
	if find("56") != nil {
		d.TipFixed = find("56").Value
	}
	if find("57") != nil {
		d.TipPercentage = find("57").Value
	}
	d.TipIndicator = tipIndicator
	if find("63") != nil {
		d.CRC = find("63").Value
	}
	return d
}

func tlvValue(t *TLV) string {
	if t == nil {
		return ""
	}
	return t.Value
}

func defaultStr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
