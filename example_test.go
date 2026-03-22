package ocrline_test

import (
	"fmt"
	"log"
	"strings"

	"github.com/karolusz/ocrline"
)

// ServiceCode is a custom type that implements Marshaler/Unmarshaler.
type ServiceCode string

func (s ServiceCode) MarshalOCR() (string, error) { return string(s), nil }
func (s *ServiceCode) UnmarshalOCR(data string) error {
	*s = ServiceCode(strings.TrimSpace(data))
	return nil
}

// RecordBase demonstrates embedded struct composition.
type RecordBase struct {
	FormatCode  string      `ocr:"0:2"`
	ServiceCode ServiceCode `ocr:"2:4"`
	TxType      string      `ocr:"4:6"`
	RecordType  int         `ocr:"6:8"`
}

// TransmissionStart demonstrates a complete AvtaleGiro record.
type TransmissionStart struct {
	RecordBase
	DataTransmitter    string `ocr:"8:16"`
	TransmissionNumber string `ocr:"16:23"`
	DataRecipient      string `ocr:"23:31"`
}

func Example_unmarshal() {
	line := "NY000010555555551000081000080800000000000000000000000000000000000000000000000000"

	var record TransmissionStart
	if err := ocrline.Unmarshal(line, &record); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Format: %s\n", record.FormatCode)
	fmt.Printf("Service: %s\n", record.ServiceCode)
	fmt.Printf("Record Type: %d\n", record.RecordType)
	fmt.Printf("Transmitter: %s\n", record.DataTransmitter)
	fmt.Printf("Number: %s\n", record.TransmissionNumber)
	fmt.Printf("Recipient: %s\n", record.DataRecipient)
	// Output:
	// Format: NY
	// Service: 00
	// Record Type: 10
	// Transmitter: 55555555
	// Number: 1000081
	// Recipient: 00008080
}

func Example_marshal() {
	record := TransmissionStart{
		RecordBase: RecordBase{
			FormatCode:  "NY",
			ServiceCode: "00",
			TxType:      "00",
			RecordType:  10,
		},
		DataTransmitter:    "55555555",
		TransmissionNumber: "1000081",
		DataRecipient:      "00008080",
	}

	line, err := ocrline.Marshal(&record)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(line)
	// Output:
	// NY000010555555551000081000080800000000000000000000000000000000000000000000000000
}

func Example_roundTrip() {
	original := "NY000010555555551000081000080800000000000000000000000000000000000000000000000000"

	// Unmarshal
	var record TransmissionStart
	if err := ocrline.Unmarshal(original, &record); err != nil {
		log.Fatal(err)
	}

	// Marshal back
	result, err := ocrline.Marshal(&record)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(result == original)
	// Output:
	// true
}

// PaymentClaim demonstrates AvtaleGiro payment claim with gap fills.
// No filler fields needed - gaps are handled by implementing Filler.
type PaymentClaim struct {
	RecordBase
	TransactionNumber int    `ocr:"8:15"`
	NetsDate          string `ocr:"15:21"`
	// positions 21:32 are a space-filled gap (no struct field needed)
	Amount int    `ocr:"32:49"`
	KID    string `ocr:"49:74,align-right,pad-space"`
}

// OCRFill specifies that the gap at positions 21:32 should be filled with spaces.
func (r PaymentClaim) OCRFill() []ocrline.Fill {
	return []ocrline.Fill{
		{Start: 21, End: 32, Char: ' '},
	}
}

func Example_avtaleGiroPaymentClaim() {
	line := "NY2121300000001170604           00000000000000100          008000011688373000000"

	var claim PaymentClaim
	if err := ocrline.Unmarshal(line, &claim); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Service: %s\n", claim.ServiceCode)
	fmt.Printf("Transaction: %d\n", claim.TransactionNumber)
	fmt.Printf("Amount: %d øre\n", claim.Amount)
	fmt.Printf("KID: %s\n", claim.KID)
	// Output:
	// Service: 21
	// Transaction: 1
	// Amount: 100 øre
	// KID: 008000011688373
}

func Example_customWidth() {
	type ShortRecord struct {
		Code  string `ocr:"0:2"`
		Value int    `ocr:"2:10"`
	}

	r := ShortRecord{Code: "AB", Value: 42}

	// Marshal with no padding (width = 0)
	line, err := ocrline.MarshalWidth(r, 0)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("No padding: %q (len=%d)\n", line, len(line))

	// Marshal with custom width
	line, err = ocrline.MarshalWidth(r, 40)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Width 40: %q (len=%d)\n", line, len(line))
	// Output:
	// No padding: "AB00000042" (len=10)
	// Width 40: "AB00000042000000000000000000000000000000" (len=40)
}
