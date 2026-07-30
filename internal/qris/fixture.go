package qris

// SampleStaticQRIS returns a minimal valid static QRIS for tests and examples.
func SampleStaticQRIS() string {
	return sampleStaticQRIS()
}

// sampleStaticQRIS is a minimal valid static QRIS for tests.
func sampleStaticQRIS() string {
	mai := buildTLVString([]TLV{
		makeTLV("00", "COM.QRISGATE"),
		makeTLV("01", "123456789012345"),
		makeTLV("03", "UMI"),
	})
	body := buildTLVString([]TLV{
		makeTLV("00", "01"),
		makeTLV("01", "11"),
		makeTLV("26", mai),
		makeTLV("52", "5812"),
		makeTLV("53", "360"),
		makeTLV("58", "ID"),
		makeTLV("59", "QRIS GATE"),
		makeTLV("60", "JAKARTA"),
	})
	crcInput := body + "6304"
	return crcInput + CalculateCRC16(crcInput)
}
