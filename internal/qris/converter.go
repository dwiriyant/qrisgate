package qris

import "fmt"

func buildTLVString(elements []TLV) string {
	var b []byte
	for _, el := range elements {
		value := el.Value
		if len(el.Children) > 0 {
			value = buildTLVString(el.Children)
		}
		b = append(b, el.Tag...)
		b = append(b, fmt.Sprintf("%02d", len(value))...)
		b = append(b, value...)
	}
	return string(b)
}

func makeTLV(tag, value string) TLV {
	return TLV{Tag: tag, Length: len(value), Value: value}
}

// Convert turns a static QRIS payload into dynamic by injecting amount and optional fee.
func Convert(qrisString string, options ConvertOptions) string {
	elements := ParseTLV(qrisString)
	result := make([]TLV, 0, len(elements)+4)
	amountInserted := false
	managed := map[string]struct{}{"54": {}, "55": {}, "56": {}, "57": {}, "63": {}}

	for _, el := range elements {
		if _, skip := managed[el.Tag]; skip {
			continue
		}
		if el.Tag == "01" {
			result = append(result, makeTLV("01", "12"))
			continue
		}
		if el.Tag == "58" && !amountInserted {
			result = append(result, makeTLV("54", fmt.Sprintf("%d", options.Amount)))
			if options.Fee != nil {
				if options.Fee.Type == "fixed" {
					result = append(result, makeTLV("55", "02"))
					result = append(result, makeTLV("56", fmt.Sprintf("%g", options.Fee.Value)))
				} else {
					result = append(result, makeTLV("55", "03"))
					result = append(result, makeTLV("57", fmt.Sprintf("%g", options.Fee.Value)))
				}
			}
			amountInserted = true
		}
		result = append(result, el)
	}

	withoutCRC := buildTLVString(result)
	crcInput := withoutCRC + "6304"
	return crcInput + CalculateCRC16(crcInput)
}
