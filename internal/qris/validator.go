package qris

import (
	"strconv"
	"strings"
)

// Validate checks structural correctness of a QRIS EMVCo string.
func Validate(qrisString string) ValidationResult {
	var errors []string
	if strings.TrimSpace(qrisString) == "" {
		return ValidationResult{Valid: false, Errors: []string{"QRIS string is empty"}}
	}
	str := strings.TrimSpace(qrisString)
	if !strings.HasPrefix(str, "000201") {
		errors = append(errors, `QRIS must start with Payload Format Indicator "000201"`)
	}
	if len(str) < 20 {
		errors = append(errors, "QRIS string is too short")
		return ValidationResult{Valid: false, Errors: errors}
	}

	dataWithoutCRC := str[:len(str)-4]
	declaredCRC := str[len(str)-4:]
	if CalculateCRC16(dataWithoutCRC) != strings.ToUpper(declaredCRC) {
		errors = append(errors, "CRC mismatch")
	}

	elements := ParseTLV(str)
	if len(elements) == 0 {
		errors = append(errors, "Failed to parse any TLV elements")
		return ValidationResult{Valid: false, Errors: errors}
	}

	tags := make(map[string]struct{}, len(elements))
	for _, e := range elements {
		tags[e.Tag] = struct{}{}
	}
	required := []struct {
		tag, name string
	}{
		{"00", "Payload Format Indicator"},
		{"01", "Point of Initiation Method"},
		{"52", "Merchant Category Code"},
		{"53", "Transaction Currency"},
		{"58", "Country Code"},
		{"59", "Merchant Name"},
		{"60", "Merchant City"},
		{"63", "CRC"},
	}
	for _, req := range required {
		if _, ok := tags[req.tag]; !ok {
			errors = append(errors, "Missing required tag "+req.tag+" ("+req.name+")")
		}
	}

	for _, e := range elements {
		if e.Tag == "01" && e.Value != "11" && e.Value != "12" {
			errors = append(errors, `Invalid Point of Initiation Method: "`+e.Value+`" (must be "11" or "12")`)
		}
	}

	hasMerchant := false
	for _, e := range elements {
		n, err := strconv.Atoi(e.Tag)
		if err == nil && n >= 26 && n <= 51 {
			hasMerchant = true
			break
		}
	}
	if !hasMerchant {
		errors = append(errors, "No Merchant Account Information found (tags 26-51)")
	}

	return ValidationResult{Valid: len(errors) == 0, Errors: errors}
}
