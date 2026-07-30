package qris

// CalculateCRC16 computes CRC16-CCITT for EMVCo QRIS payloads (poly 0x1021, init 0xFFFF).
func CalculateCRC16(s string) string {
	crc := uint16(0xffff)
	for i := 0; i < len(s); i++ {
		crc ^= uint16(s[i]) << 8
		for j := 0; j < 8; j++ {
			if crc&0x8000 != 0 {
				crc = ((crc << 1) ^ 0x1021) & 0xffff
			} else {
				crc = (crc << 1) & 0xffff
			}
		}
	}
	return upperHex4(crc)
}

func upperHex4(v uint16) string {
	const hex = "0123456789ABCDEF"
	return string([]byte{hex[v>>12], hex[(v>>8)&0xf], hex[(v>>4)&0xf], hex[v&0xf]})
}
