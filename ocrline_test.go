package ocrline

import (
	"fmt"
	"strings"
	"testing"
)

// --- Test types ---

// Simulates AvtaleGiro/OCR Giro named types
type (
	Numeric      string
	NumericInt   int
	Alphanumeric string
)

// Custom type with Marshaler/Unmarshaler
type ServiceCode string

func (s ServiceCode) MarshalOCR() (string, error) {
	return string(s), nil
}

func (s *ServiceCode) UnmarshalOCR(data string) error {
	*s = ServiceCode(strings.TrimSpace(data))
	return nil
}

// Custom type with validation
type RecordType int

func (r RecordType) MarshalOCR() (string, error) {
	return fmt.Sprintf("%d", r), nil
}

func (r *RecordType) UnmarshalOCR(data string) error {
	trimmed := strings.TrimSpace(data)
	var n int
	_, err := fmt.Sscanf(trimmed, "%d", &n)
	if err != nil {
		return fmt.Errorf("invalid record type %q: %w", data, err)
	}
	*r = RecordType(n)
	return nil
}

// --- Test structs ---

type RecordBaseInfo struct {
	FormatCode  string      `ocr:"0:2"`
	ServiceCode ServiceCode `ocr:"2:4"`
	Type        string      `ocr:"4:6"`
	RecordType  RecordType  `ocr:"6:8"`
}

type TransmissionStart struct {
	RecordBaseInfo
	DataTransmitter    string `ocr:"8:16"`
	TransmissionNumber string `ocr:"16:23"`
	DataRecipient      string `ocr:"23:31"`
}

type TransactionItemOne struct {
	RecordBaseInfo
	TransactionNumber int    `ocr:"8:15"`
	NetsDate          string `ocr:"15:21"`
	Centre            string `ocr:"21:23"`
	DayCode           string `ocr:"23:25"`
	PartialSettlement string `ocr:"25:26"`
	SerialNumber      string `ocr:"26:31"`
	Sign              string `ocr:"31:32"`
	Amount            int    `ocr:"32:49"`
	KID               string `ocr:"49:74,align-right,pad-space"`
	CardDrawer        string `ocr:"74:76"`
}

type PaymentClaimLineOne struct {
	RecordBaseInfo
	TransactionNumber int    `ocr:"8:15"`
	NetsDate          string `ocr:"15:21"`
	// gap at 21:32 filled with spaces via OCRFill()
	Amount int    `ocr:"32:49"`
	KID    string `ocr:"49:74,align-right,pad-space"`
}

// OCRFill implements the Filler interface to fill gaps with the correct characters.
func (r PaymentClaimLineOne) OCRFill() []Fill {
	return []Fill{
		{Start: 21, End: 32, Char: ' '},
	}
}

type SimpleRecord struct {
	Code  string `ocr:"0:2"`
	Name  string `ocr:"2:12"`
	Value int    `ocr:"12:20"`
}

type RecordWithBool struct {
	Code   string `ocr:"0:2"`
	Active bool   `ocr:"2:3"`
}

type RecordWithOmitempty struct {
	Code    string `ocr:"0:2"`
	OptDate string `ocr:"2:8,omitempty,pad-zero"`
	Amount  int    `ocr:"8:15"`
}

type RecordWithUint struct {
	Code  string `ocr:"0:2"`
	Count uint   `ocr:"2:10"`
}

// --- Unmarshal Tests ---

func TestUnmarshal_SimpleRecord(t *testing.T) {
	line := "ABTEST NAME 00012345" + strings.Repeat("0", 60)
	var r SimpleRecord
	err := Unmarshal(line, &r)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if r.Code != "AB" {
		t.Errorf("Code = %q, want %q", r.Code, "AB")
	}
	if r.Name != "TEST NAME" {
		t.Errorf("Name = %q, want %q", r.Name, "TEST NAME")
	}
	if r.Value != 12345 {
		t.Errorf("Value = %d, want %d", r.Value, 12345)
	}
}

func TestUnmarshal_TransmissionStart(t *testing.T) {
	line := "NY000010555555551000081000080800000000000000000000000000000000000000000000000000"
	var r TransmissionStart
	err := Unmarshal(line, &r)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if r.FormatCode != "NY" {
		t.Errorf("FormatCode = %q, want %q", r.FormatCode, "NY")
	}
	if r.ServiceCode != "00" {
		t.Errorf("ServiceCode = %q, want %q", r.ServiceCode, "00")
	}
	if r.Type != "00" {
		t.Errorf("Type = %q, want %q", r.Type, "00")
	}
	if r.RecordType != 10 {
		t.Errorf("RecordType = %d, want %d", r.RecordType, 10)
	}
	if r.DataTransmitter != "55555555" {
		t.Errorf("DataTransmitter = %q, want %q", r.DataTransmitter, "55555555")
	}
	if r.TransmissionNumber != "1000081" {
		t.Errorf("TransmissionNumber = %q, want %q", r.TransmissionNumber, "1000081")
	}
	if r.DataRecipient != "00008080" {
		t.Errorf("DataRecipient = %q, want %q", r.DataRecipient, "00008080")
	}
}

func TestUnmarshal_TransactionItemOne(t *testing.T) {
	// OCR Giro transaction line (80 chars)
	// Positions: NY(0:2) 09(2:4) 10(4:6) 30(6:8) 0000001(8:15) 170604(15:21) 01(21:23) 02(23:25) 0(25:26) 00010(26:31) 0(31:32) 00000000000000100(32:49) ...
	line := "NY091030000000117060401020001000000000000000010000000000001688373000000"
	line += strings.Repeat("0", 80-len(line))
	var r TransactionItemOne
	err := Unmarshal(line, &r)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if r.FormatCode != "NY" {
		t.Errorf("FormatCode = %q, want %q", r.FormatCode, "NY")
	}
	if r.ServiceCode != "09" {
		t.Errorf("ServiceCode = %q, want %q", r.ServiceCode, "09")
	}
	if r.TransactionNumber != 1 {
		t.Errorf("TransactionNumber = %d, want %d", r.TransactionNumber, 1)
	}
	// Amount at 32:49 = "00000000000001000" = 1000 (in øre)
	if r.Amount != 1000 {
		t.Errorf("Amount = %d, want %d", r.Amount, 1000)
	}
}

func TestUnmarshal_AvtaleGiroPaymentClaim(t *testing.T) {
	line := "NY2121300000001170604           00000000000000100          008000011688373000000"
	var r PaymentClaimLineOne
	err := Unmarshal(line, &r)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if r.ServiceCode != "21" {
		t.Errorf("ServiceCode = %q, want %q", r.ServiceCode, "21")
	}
	if r.TransactionNumber != 1 {
		t.Errorf("TransactionNumber = %d, want %d", r.TransactionNumber, 1)
	}
	if r.Amount != 100 {
		t.Errorf("Amount = %d, want %d", r.Amount, 100)
	}
	if r.KID != "008000011688373" {
		t.Errorf("KID = %q, want %q", r.KID, "008000011688373")
	}
}

func TestUnmarshal_Bool(t *testing.T) {
	line := "AB1" + strings.Repeat("0", 77)
	var r RecordWithBool
	err := Unmarshal(line, &r)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if !r.Active {
		t.Error("Active = false, want true")
	}
}

func TestUnmarshal_Uint(t *testing.T) {
	line := "AB00000042" + strings.Repeat("0", 70)
	var r RecordWithUint
	err := Unmarshal(line, &r)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if r.Count != 42 {
		t.Errorf("Count = %d, want %d", r.Count, 42)
	}
}

func TestUnmarshal_NilPointer(t *testing.T) {
	err := Unmarshal("test", nil)
	if err == nil {
		t.Fatal("expected error for nil pointer")
	}
	if _, ok := err.(*InvalidUnmarshalError); !ok {
		t.Errorf("expected InvalidUnmarshalError, got %T", err)
	}
}

func TestUnmarshal_NonPointer(t *testing.T) {
	var r SimpleRecord
	err := Unmarshal("test", r)
	if err == nil {
		t.Fatal("expected error for non-pointer")
	}
}

func TestUnmarshal_RangeError(t *testing.T) {
	line := "AB" // too short
	var r SimpleRecord
	err := Unmarshal(line, &r)
	if err == nil {
		t.Fatal("expected error for short line")
	}
	if _, ok := err.(*UnmarshalRangeError); !ok {
		t.Errorf("expected UnmarshalRangeError, got %T: %v", err, err)
	}
}

// --- Marshal Tests ---

func TestMarshal_SimpleRecord(t *testing.T) {
	r := SimpleRecord{
		Code:  "AB",
		Name:  "HELLO",
		Value: 42,
	}
	line, err := Marshal(r)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if len(line) != 80 {
		t.Errorf("line length = %d, want 80", len(line))
	}
	// Code at 0:2
	if line[0:2] != "AB" {
		t.Errorf("Code = %q, want %q", line[0:2], "AB")
	}
	// Name at 2:12, left-aligned, space-padded
	if line[2:12] != "HELLO     " {
		t.Errorf("Name = %q, want %q", line[2:12], "HELLO     ")
	}
	// Value at 12:20, right-aligned, zero-padded
	if line[12:20] != "00000042" {
		t.Errorf("Value = %q, want %q", line[12:20], "00000042")
	}
}

func TestMarshal_TransmissionStart(t *testing.T) {
	r := TransmissionStart{
		RecordBaseInfo: RecordBaseInfo{
			FormatCode:  "NY",
			ServiceCode: "00",
			Type:        "00",
			RecordType:  10,
		},
		DataTransmitter:    "55555555",
		TransmissionNumber: "1000081",
		DataRecipient:      "00008080",
	}
	line, err := Marshal(r)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	expected := "NY000010555555551000081000080800000000000000000000000000000000000000000000000000"
	if line != expected {
		t.Errorf("Marshal result:\n  got  %q\n  want %q", line, expected)
	}
}

func TestMarshal_WithFillerInterface(t *testing.T) {
	r := PaymentClaimLineOne{
		RecordBaseInfo: RecordBaseInfo{
			FormatCode:  "NY",
			ServiceCode: "21",
			Type:        "21",
			RecordType:  30,
		},
		TransactionNumber: 1,
		NetsDate:          "170604",
		Amount:            100,
		KID:               "008000011688373",
	}
	line, err := Marshal(r)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	// Gap at 21:32 should be space-filled via OCRFill()
	gap := line[21:32]
	if gap != "           " {
		t.Errorf("gap [21:32] = %q, want 11 spaces", gap)
	}
	// Gap at 74:80 should be default '0' fill (not covered by OCRFill)
	tail := line[74:80]
	if tail != "000000" {
		t.Errorf("gap [74:80] = %q, want %q", tail, "000000")
	}
}

func TestMarshal_Omitempty(t *testing.T) {
	r := RecordWithOmitempty{
		Code:    "AB",
		OptDate: "",
		Amount:  100,
	}
	line, err := Marshal(r)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	// OptDate at 2:8 with omitempty and pad-zero should be "000000"
	if line[2:8] != "000000" {
		t.Errorf("OptDate = %q, want %q", line[2:8], "000000")
	}
}

func TestMarshal_Bool(t *testing.T) {
	r := RecordWithBool{Code: "AB", Active: true}
	line, err := Marshal(&r)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if line[2:3] != "1" {
		t.Errorf("Active = %q, want %q", line[2:3], "1")
	}
}

func TestMarshal_Uint(t *testing.T) {
	r := RecordWithUint{Code: "AB", Count: 42}
	line, err := Marshal(&r)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if line[2:10] != "00000042" {
		t.Errorf("Count = %q, want %q", line[2:10], "00000042")
	}
}

func TestMarshal_NilPointer(t *testing.T) {
	_, err := Marshal((*SimpleRecord)(nil))
	if err == nil {
		t.Fatal("expected error for nil pointer")
	}
}

func TestMarshalWidth_Zero(t *testing.T) {
	r := SimpleRecord{Code: "AB", Name: "HI", Value: 1}
	line, err := MarshalWidth(&r, 0)
	if err != nil {
		t.Fatalf("MarshalWidth failed: %v", err)
	}
	// Should be exactly as wide as the rightmost field (20)
	if len(line) != 20 {
		t.Errorf("line length = %d, want 20", len(line))
	}
}

func TestMarshalWidth_Custom(t *testing.T) {
	r := SimpleRecord{Code: "AB", Name: "HI", Value: 1}
	line, err := MarshalWidth(&r, 100)
	if err != nil {
		t.Fatalf("MarshalWidth failed: %v", err)
	}
	if len(line) != 100 {
		t.Errorf("line length = %d, want 100", len(line))
	}
}

// --- Round-trip Tests ---

func TestRoundTrip_TransmissionStart(t *testing.T) {
	original := "NY000010555555551000081000080800000000000000000000000000000000000000000000000000"
	var r TransmissionStart
	if err := Unmarshal(original, &r); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	result, err := Marshal(&r)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if result != original {
		t.Errorf("Round-trip failed:\n  got  %q\n  want %q", result, original)
	}
}

func TestRoundTrip_SimpleRecord(t *testing.T) {
	r := SimpleRecord{Code: "XY", Name: "TESTNAME", Value: 99999}
	line, err := Marshal(&r)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var r2 SimpleRecord
	if err := Unmarshal(line, &r2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if r2.Code != r.Code {
		t.Errorf("Code = %q, want %q", r2.Code, r.Code)
	}
	if r2.Name != r.Name {
		t.Errorf("Name = %q, want %q", r2.Name, r.Name)
	}
	if r2.Value != r.Value {
		t.Errorf("Value = %d, want %d", r2.Value, r.Value)
	}
}

// --- Custom Marshaler/Unmarshaler Tests ---

func TestCustomMarshaler(t *testing.T) {
	r := TransmissionStart{
		RecordBaseInfo: RecordBaseInfo{
			FormatCode:  "NY",
			ServiceCode: "09",
			Type:        "00",
			RecordType:  10,
		},
		DataTransmitter:    "12345678",
		TransmissionNumber: "0000001",
		DataRecipient:      "87654321",
	}
	line, err := Marshal(&r)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if line[2:4] != "09" {
		t.Errorf("ServiceCode = %q, want %q", line[2:4], "09")
	}
	if line[6:8] != "10" {
		t.Errorf("RecordType = %q, want %q", line[6:8], "10")
	}
}

func TestCustomUnmarshaler(t *testing.T) {
	line := "NY090010123456780000001876543210000000000000000000000000000000000000000000000000"
	var r TransmissionStart
	err := Unmarshal(line, &r)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if r.ServiceCode != "09" {
		t.Errorf("ServiceCode = %q, want %q", r.ServiceCode, "09")
	}
	if r.RecordType != 10 {
		t.Errorf("RecordType = %d, want %d", r.RecordType, 10)
	}
}

// --- Tag Error Tests ---

type BadTagRecord struct {
	Field string `ocr:"invalid"`
}

func TestUnmarshal_BadTag(t *testing.T) {
	line := strings.Repeat("0", 80)
	var r BadTagRecord
	err := Unmarshal(line, &r)
	if err == nil {
		t.Fatal("expected error for bad tag")
	}
	if _, ok := err.(*TagError); !ok {
		t.Errorf("expected TagError, got %T: %v", err, err)
	}
}

func TestMarshal_BadTag(t *testing.T) {
	r := BadTagRecord{Field: "test"}
	_, err := Marshal(&r)
	if err == nil {
		t.Fatal("expected error for bad tag")
	}
	if _, ok := err.(*TagError); !ok {
		t.Errorf("expected TagError, got %T: %v", err, err)
	}
}

// --- Overlap Detection Tests ---

type OverlappingRecord struct {
	A string `ocr:"0:10"`
	B string `ocr:"5:15"` // overlaps with A
}

func TestMarshal_Overlap(t *testing.T) {
	r := OverlappingRecord{A: "HELLO", B: "WORLD"}
	_, err := Marshal(r)
	if err == nil {
		t.Fatal("expected error for overlapping tags")
	}
	if _, ok := err.(*OverlapError); !ok {
		t.Errorf("expected OverlapError, got %T: %v", err, err)
	}
}

func TestUnmarshal_Overlap(t *testing.T) {
	line := strings.Repeat("A", 80)
	var r OverlappingRecord
	err := Unmarshal(line, &r)
	if err == nil {
		t.Fatal("expected error for overlapping tags")
	}
	if _, ok := err.(*OverlapError); !ok {
		t.Errorf("expected OverlapError, got %T: %v", err, err)
	}
}

type ExactlyAdjacentRecord struct {
	A string `ocr:"0:10"`
	B string `ocr:"10:20"` // adjacent, not overlapping
}

func TestMarshal_Adjacent_NoError(t *testing.T) {
	r := ExactlyAdjacentRecord{A: "HELLO", B: "WORLD"}
	_, err := MarshalWidth(r, 20)
	if err != nil {
		t.Fatalf("adjacent tags should not error: %v", err)
	}
}

type SameStartRecord struct {
	A string `ocr:"0:5"`
	B string `ocr:"0:10"` // same start, different end
}

func TestMarshal_SameStart(t *testing.T) {
	r := SameStartRecord{A: "HI", B: "HELLO"}
	_, err := Marshal(r)
	if err == nil {
		t.Fatal("expected error for same-start overlapping tags")
	}
	if _, ok := err.(*OverlapError); !ok {
		t.Errorf("expected OverlapError, got %T: %v", err, err)
	}
}

type OverlapInEmbeddedRecord struct {
	RecordBaseInfo        // uses 0:2, 2:4, 4:6, 6:8
	Extra          string `ocr:"3:10"` // overlaps with ServiceCode (2:4)
}

func TestMarshal_OverlapWithEmbedded(t *testing.T) {
	r := OverlapInEmbeddedRecord{
		RecordBaseInfo: RecordBaseInfo{FormatCode: "NY", ServiceCode: "00", Type: "00", RecordType: 10},
		Extra:          "TEST",
	}
	_, err := Marshal(r)
	if err == nil {
		t.Fatal("expected error for overlap with embedded struct field")
	}
	if _, ok := err.(*OverlapError); !ok {
		t.Errorf("expected OverlapError, got %T: %v", err, err)
	}
}

// Out-of-order tags should work fine
type OutOfOrderRecord struct {
	B string `ocr:"10:20"`
	A string `ocr:"0:10"`
}

func TestMarshal_OutOfOrder(t *testing.T) {
	r := OutOfOrderRecord{B: "WORLD", A: "HELLO"}
	line, err := MarshalWidth(r, 20)
	if err != nil {
		t.Fatalf("out-of-order tags should work: %v", err)
	}
	if line[0:10] != "HELLO     " {
		t.Errorf("A = %q, want %q", line[0:10], "HELLO     ")
	}
	if line[10:20] != "WORLD     " {
		t.Errorf("B = %q, want %q", line[10:20], "WORLD     ")
	}
}

func TestUnmarshal_OutOfOrder(t *testing.T) {
	line := "HELLO     WORLD     "
	var r OutOfOrderRecord
	err := Unmarshal(line, &r)
	if err != nil {
		t.Fatalf("out-of-order tags should work: %v", err)
	}
	if r.A != "HELLO" {
		t.Errorf("A = %q, want %q", r.A, "HELLO")
	}
	if r.B != "WORLD" {
		t.Errorf("B = %q, want %q", r.B, "WORLD")
	}
}

// --- Named Type Tests ---

type NamedTypeRecord struct {
	Code   Alphanumeric `ocr:"0:4"`
	Number Numeric      `ocr:"4:10"`
	Count  NumericInt   `ocr:"10:15"`
}

func TestUnmarshal_NamedTypes(t *testing.T) {
	line := "ABCD000042" + "00100" + strings.Repeat("0", 65)
	var r NamedTypeRecord
	err := Unmarshal(line, &r)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if r.Code != "ABCD" {
		t.Errorf("Code = %q, want %q", r.Code, "ABCD")
	}
	if r.Number != "000042" {
		t.Errorf("Number = %q, want %q", r.Number, "000042")
	}
	if r.Count != 100 {
		t.Errorf("Count = %d, want %d", r.Count, 100)
	}
}

func TestMarshal_NamedTypes(t *testing.T) {
	r := NamedTypeRecord{
		Code:   "AB",
		Number: "42",
		Count:  100,
	}
	line, err := Marshal(&r)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	// Alphanumeric: left-aligned, space-padded
	if line[0:4] != "AB  " {
		t.Errorf("Code = %q, want %q", line[0:4], "AB  ")
	}
	// Numeric (string): left-aligned, space-padded (string default)
	if line[4:10] != "42    " {
		t.Errorf("Number = %q, want %q", line[4:10], "42    ")
	}
	// NumericInt (int): right-aligned, zero-padded
	if line[10:15] != "00100" {
		t.Errorf("Count = %q, want %q", line[10:15], "00100")
	}
}

// --- Alignment/Padding Override Tests ---

type AlignOverrideRecord struct {
	// String field with right-align and zero-pad (override defaults)
	KID string `ocr:"0:10,align-right,pad-zero"`
	// Int field with left-align and space-pad (override defaults)
	Num int `ocr:"10:18,align-left,pad-space"`
}

func TestMarshal_AlignmentOverride(t *testing.T) {
	r := AlignOverrideRecord{KID: "12345", Num: 42}
	line, err := MarshalWidth(&r, 18)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if line[0:10] != "0000012345" {
		t.Errorf("KID = %q, want %q", line[0:10], "0000012345")
	}
	if line[10:18] != "42      " {
		t.Errorf("Num = %q, want %q", line[10:18], "42      ")
	}
}

func TestUnmarshal_AlignmentOverride(t *testing.T) {
	line := "000001234542      "
	var r AlignOverrideRecord
	err := Unmarshal(line, &r)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	// String fields are trimmed
	if r.KID != "0000012345" {
		t.Errorf("KID = %q, want %q", r.KID, "0000012345")
	}
	if r.Num != 42 {
		t.Errorf("Num = %d, want %d", r.Num, 42)
	}
}

// --- Embedded pointer struct test ---

type EmbeddedBase struct {
	A string `ocr:"0:2"`
	B int    `ocr:"2:5"`
}

type WithEmbeddedPointer struct {
	*EmbeddedBase
	C string `ocr:"5:10"`
}

func TestUnmarshal_EmbeddedPointer(t *testing.T) {
	line := "AB042HELLO" + strings.Repeat("0", 70)
	var r WithEmbeddedPointer
	err := Unmarshal(line, &r)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if r.EmbeddedBase == nil {
		t.Fatal("EmbeddedBase is nil, want non-nil (should be auto-allocated)")
	}
	if r.A != "AB" {
		t.Errorf("A = %q, want %q", r.A, "AB")
	}
	if r.B != 42 {
		t.Errorf("B = %d, want %d", r.B, 42)
	}
	if r.C != "HELLO" {
		t.Errorf("C = %q, want %q", r.C, "HELLO")
	}
}

func TestMarshal_EmbeddedPointer_NonNil(t *testing.T) {
	r := WithEmbeddedPointer{
		EmbeddedBase: &EmbeddedBase{A: "XY", B: 99},
		C:            "WORLD",
	}
	line, err := MarshalWidth(r, 10)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if line[0:2] != "XY" {
		t.Errorf("A = %q, want %q", line[0:2], "XY")
	}
	if line[2:5] != "099" {
		t.Errorf("B = %q, want %q", line[2:5], "099")
	}
	if line[5:10] != "WORLD" {
		t.Errorf("C = %q, want %q", line[5:10], "WORLD")
	}
}

func TestMarshal_EmbeddedPointer_Nil(t *testing.T) {
	r := WithEmbeddedPointer{
		EmbeddedBase: nil,
		C:            "HELLO",
	}
	line, err := MarshalWidth(r, 10)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	// Nil embedded pointer: positions 0:5 should be default fill '0'
	if line[0:5] != "00000" {
		t.Errorf("positions [0:5] = %q, want %q", line[0:5], "00000")
	}
	if line[5:10] != "HELLO" {
		t.Errorf("C = %q, want %q", line[5:10], "HELLO")
	}
}

// --- Unexported field test ---

type RecordWithUnexported struct {
	Code    string `ocr:"0:2"`
	secret  string `ocr:"2:10"` //nolint:unused // intentionally unexported for test
	Visible int    `ocr:"10:15"`
}

func TestMarshal_UnexportedFieldSkipped(t *testing.T) {
	r := RecordWithUnexported{Code: "AB", Visible: 42}
	line, err := MarshalWidth(r, 15)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if line[0:2] != "AB" {
		t.Errorf("Code = %q, want %q", line[0:2], "AB")
	}
	// positions 2:10 should be default fill '0' (unexported field skipped)
	if line[2:10] != "00000000" {
		t.Errorf("gap [2:10] = %q, want %q (unexported field should be skipped)", line[2:10], "00000000")
	}
	if line[10:15] != "00042" {
		t.Errorf("Visible = %q, want %q", line[10:15], "00042")
	}
}

func TestUnmarshal_UnexportedFieldSkipped(t *testing.T) {
	line := "ABXXXXXXXX00042"
	var r RecordWithUnexported
	err := Unmarshal(line, &r)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if r.Code != "AB" {
		t.Errorf("Code = %q, want %q", r.Code, "AB")
	}
	// secret should remain zero value (skipped)
	if r.secret != "" {
		t.Errorf("secret = %q, want empty (unexported field should be skipped)", r.secret)
	}
	if r.Visible != 42 {
		t.Errorf("Visible = %d, want %d", r.Visible, 42)
	}
}

// --- Error message tests ---

func TestErrorMessages(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "InvalidMarshalError nil",
			err:  &InvalidMarshalError{Type: nil},
			want: "ocrline: Marshal(nil)",
		},
		{
			name: "InvalidUnmarshalError nil",
			err:  &InvalidUnmarshalError{Type: nil},
			want: "ocrline: Unmarshal(nil)",
		},
		{
			name: "TagError",
			err:  &TagError{Field: "Foo", Tag: "bad", Err: fmt.Errorf("oops")},
			want: `ocrline: invalid tag on field Foo: "bad": oops`,
		},
		{
			name: "UnmarshalRangeError",
			err:  &UnmarshalRangeError{Field: "Bar", Start: 0, End: 100, LineWidth: 80},
			want: "ocrline: field Bar range [0:100] exceeds line width 80",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- Benchmark ---

// --- Pointer Tests ---

type RecordWithPointer struct {
	Code    string  `ocr:"0:2"`
	OptName *string `ocr:"2:12"`
	OptNum  *int    `ocr:"12:20"`
}

func TestMarshal_Pointer_NonNil(t *testing.T) {
	name := "HELLO"
	num := 42
	r := RecordWithPointer{Code: "AB", OptName: &name, OptNum: &num}
	line, err := MarshalWidth(&r, 20)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if line[2:12] != "HELLO     " {
		t.Errorf("OptName = %q, want %q", line[2:12], "HELLO     ")
	}
	if line[12:20] != "00000042" {
		t.Errorf("OptNum = %q, want %q", line[12:20], "00000042")
	}
}

func TestMarshal_Pointer_Nil(t *testing.T) {
	r := RecordWithPointer{Code: "AB", OptName: nil, OptNum: nil}
	line, err := MarshalWidth(&r, 20)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	// nil *string -> empty -> left-aligned, space-padded
	if line[2:12] != "          " {
		t.Errorf("OptName = %q, want 10 spaces", line[2:12])
	}
	// nil *int -> empty -> right-aligned, zero-padded
	if line[12:20] != "00000000" {
		t.Errorf("OptNum = %q, want %q", line[12:20], "00000000")
	}
}

func TestUnmarshal_Pointer(t *testing.T) {
	line := "ABTEST NAME 00000042"
	var r RecordWithPointer
	err := Unmarshal(line, &r)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if r.OptName == nil {
		t.Fatal("OptName is nil, want non-nil")
	}
	if *r.OptName != "TEST NAME" {
		t.Errorf("OptName = %q, want %q", *r.OptName, "TEST NAME")
	}
	if r.OptNum == nil {
		t.Fatal("OptNum is nil, want non-nil")
	}
	if *r.OptNum != 42 {
		t.Errorf("OptNum = %d, want %d", *r.OptNum, 42)
	}
}

// --- Byte/Rune alias tests (covered by uint8/int32 but verify explicitly) ---

func TestMarshal_ByteField(t *testing.T) {
	type R struct {
		Code string `ocr:"0:2"`
		B    byte   `ocr:"2:5"`
	}
	r := R{Code: "AB", B: 65} // 'A' = 65
	line, err := MarshalWidth(&r, 5)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if line[2:5] != "065" {
		t.Errorf("B = %q, want %q", line[2:5], "065")
	}
}

func TestUnmarshal_ByteField(t *testing.T) {
	type R struct {
		Code string `ocr:"0:2"`
		B    byte   `ocr:"2:5"`
	}
	line := "AB065"
	var r R
	err := Unmarshal(line, &r)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if r.B != 65 {
		t.Errorf("B = %d, want %d", r.B, 65)
	}
}

// --- Coverage gap tests ---

// OverlapError.Error()
func TestOverlapError_String(t *testing.T) {
	e := &OverlapError{
		Field1: "A", Start1: 0, End1: 10,
		Field2: "B", Start2: 5, End2: 15,
	}
	want := "ocrline: fields A [0:10] and B [5:15] overlap"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// InvalidMarshalError - non-struct (not pointer)
func TestMarshal_NonStruct(t *testing.T) {
	_, err := Marshal(42)
	if err == nil {
		t.Fatal("expected error for non-struct")
	}
	e, ok := err.(*InvalidMarshalError)
	if !ok {
		t.Fatalf("expected InvalidMarshalError, got %T", err)
	}
	if !strings.Contains(e.Error(), "non-struct") {
		t.Errorf("error = %q, want to contain 'non-struct'", e.Error())
	}
}

// InvalidMarshalError - nil with no type
func TestMarshal_NilInterface(t *testing.T) {
	_, err := Marshal(nil)
	if err == nil {
		t.Fatal("expected error for nil")
	}
}

// InvalidUnmarshalError - non-pointer
func TestUnmarshalError_NonPointer(t *testing.T) {
	err := Unmarshal("test", SimpleRecord{})
	if err == nil {
		t.Fatal("expected error")
	}
	e, ok := err.(*InvalidUnmarshalError)
	if !ok {
		t.Fatalf("expected InvalidUnmarshalError, got %T", err)
	}
	if !strings.Contains(e.Error(), "non-pointer") {
		t.Errorf("error = %q, want to contain 'non-pointer'", e.Error())
	}
}

// InvalidUnmarshalError - pointer to non-struct
func TestUnmarshal_PointerToNonStruct(t *testing.T) {
	var n int
	err := Unmarshal("test", &n)
	if err == nil {
		t.Fatal("expected error for pointer to non-struct")
	}
	e, ok := err.(*InvalidUnmarshalError)
	if !ok {
		t.Fatalf("expected InvalidUnmarshalError, got %T", err)
	}
	if !strings.Contains(e.Error(), "nil") {
		t.Errorf("error = %q, want to contain 'nil'", e.Error())
	}
}

// MarshalFieldError and Unwrap
func TestMarshalFieldError(t *testing.T) {
	inner := fmt.Errorf("boom")
	e := &MarshalFieldError{Field: "Foo", Err: inner}
	if !strings.Contains(e.Error(), "Foo") {
		t.Errorf("error = %q, want to contain 'Foo'", e.Error())
	}
	if e.Unwrap() != inner {
		t.Error("Unwrap() did not return inner error")
	}
}

// UnmarshalFieldError and Unwrap
func TestUnmarshalFieldError(t *testing.T) {
	inner := fmt.Errorf("boom")
	e := &UnmarshalFieldError{Field: "Bar", Err: inner}
	if !strings.Contains(e.Error(), "Bar") {
		t.Errorf("error = %q, want to contain 'Bar'", e.Error())
	}
	if e.Unwrap() != inner {
		t.Error("Unwrap() did not return inner error")
	}
}

// TagError Unwrap
func TestTagError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("bad")
	e := &TagError{Field: "X", Tag: "y", Err: inner}
	if e.Unwrap() != inner {
		t.Error("Unwrap() did not return inner error")
	}
}

// Custom marshaler that returns an error
type FailMarshaler struct{}

func (f FailMarshaler) MarshalOCR() (string, error) {
	return "", fmt.Errorf("marshal failed")
}

type RecordWithFailMarshaler struct {
	Code string        `ocr:"0:2"`
	Bad  FailMarshaler `ocr:"2:10"`
}

func TestMarshal_MarshalerError(t *testing.T) {
	r := RecordWithFailMarshaler{Code: "AB"}
	_, err := Marshal(r)
	if err == nil {
		t.Fatal("expected error from custom marshaler")
	}
	if _, ok := err.(*MarshalFieldError); !ok {
		t.Errorf("expected MarshalFieldError, got %T: %v", err, err)
	}
}

// Custom unmarshaler that returns an error (pointer receiver)
type FailUnmarshaler struct{}

func (f *FailUnmarshaler) UnmarshalOCR(data string) error {
	return fmt.Errorf("unmarshal failed")
}

type RecordWithFailUnmarshaler struct {
	Code string          `ocr:"0:2"`
	Bad  FailUnmarshaler `ocr:"2:10"`
}

func TestUnmarshal_UnmarshalerError(t *testing.T) {
	line := "AB12345678" + strings.Repeat("0", 70)
	var r RecordWithFailUnmarshaler
	err := Unmarshal(line, &r)
	if err == nil {
		t.Fatal("expected error from custom unmarshaler")
	}
	if _, ok := err.(*UnmarshalFieldError); !ok {
		t.Errorf("expected UnmarshalFieldError, got %T: %v", err, err)
	}
}

// Unsupported field type in unmarshal
type RecordWithUnsupportedType struct {
	Code string    `ocr:"0:2"`
	Bad  complex64 `ocr:"2:10"`
}

func TestUnmarshal_UnsupportedType(t *testing.T) {
	line := "AB12345678" + strings.Repeat("0", 70)
	var r RecordWithUnsupportedType
	err := Unmarshal(line, &r)
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

// fieldToString default branch (unsupported type falls through to Sprintf)
func TestMarshal_UnsupportedTypeFallthrough(t *testing.T) {
	// complex64 has no explicit case, hits default fmt.Sprintf
	r := RecordWithUnsupportedType{Code: "AB", Bad: 1 + 2i}
	line, err := MarshalWidth(r, 10)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	// Should contain something (fmt.Sprintf output)
	if line[0:2] != "AB" {
		t.Errorf("Code = %q, want %q", line[0:2], "AB")
	}
}

// Filler interface with fill range exceeding line width
type RecordWithWideFill struct {
	Code string `ocr:"0:2"`
}

func (r RecordWithWideFill) OCRFill() []Fill {
	return []Fill{
		{Start: 5, End: 200, Char: ' '}, // exceeds any reasonable width
	}
}

func TestMarshal_FillerExceedsWidth(t *testing.T) {
	r := RecordWithWideFill{Code: "AB"}
	line, err := MarshalWidth(r, 10)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if line[0:2] != "AB" {
		t.Errorf("Code = %q, want %q", line[0:2], "AB")
	}
	// positions 5-9 should be spaces (clamped to width 10)
	if line[5:10] != "     " {
		t.Errorf("fill = %q, want 5 spaces", line[5:10])
	}
}

// Unmarshal with value-receiver Unmarshaler (not pointer receiver)
type ValueUnmarshaler string

func (v ValueUnmarshaler) UnmarshalOCR(data string) error {
	return nil // no-op, just testing the path is reached
}

type RecordWithValueUnmarshaler struct {
	Code string           `ocr:"0:2"`
	Val  ValueUnmarshaler `ocr:"2:10"`
}

func TestUnmarshal_ValueReceiverUnmarshaler(t *testing.T) {
	line := "ABXXXXXXXX" + strings.Repeat("0", 70)
	var r RecordWithValueUnmarshaler
	err := Unmarshal(line, &r)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

// Empty int/uint fields in unmarshal
func TestUnmarshal_EmptyIntField(t *testing.T) {
	type R struct {
		Code string `ocr:"0:2"`
		Num  int    `ocr:"2:10"`
	}
	line := "AB        " // spaces for int field
	var r R
	err := Unmarshal(line, &r)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if r.Num != 0 {
		t.Errorf("Num = %d, want 0", r.Num)
	}
}

func TestUnmarshal_EmptyUintField(t *testing.T) {
	type R struct {
		Code  string `ocr:"0:2"`
		Count uint   `ocr:"2:10"`
	}
	line := "AB        "
	var r R
	err := Unmarshal(line, &r)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if r.Count != 0 {
		t.Errorf("Count = %d, want 0", r.Count)
	}
}

// Bool marshal false
func TestMarshal_BoolFalse(t *testing.T) {
	r := RecordWithBool{Code: "AB", Active: false}
	line, err := Marshal(r)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if line[2:3] != "0" {
		t.Errorf("Active = %q, want %q", line[2:3], "0")
	}
}

// Bool unmarshal various truthy/falsy values
func TestUnmarshal_BoolVariants(t *testing.T) {
	tests := []struct {
		char string
		want bool
	}{
		{"J", true},
		{"Y", true},
		{"N", false},
		{"0", false},
	}
	for _, tt := range tests {
		line := "AB" + tt.char + strings.Repeat("0", 77)
		var r RecordWithBool
		err := Unmarshal(line, &r)
		if err != nil {
			t.Fatalf("Unmarshal(%q) failed: %v", tt.char, err)
		}
		if r.Active != tt.want {
			t.Errorf("Unmarshal(%q): Active = %v, want %v", tt.char, r.Active, tt.want)
		}
	}
}

// Bool unmarshal invalid value
func TestUnmarshal_BoolInvalid(t *testing.T) {
	line := "ABX" + strings.Repeat("0", 77)
	var r RecordWithBool
	err := Unmarshal(line, &r)
	if err == nil {
		t.Fatal("expected error for invalid bool value")
	}
}

// Invalid int parse
func TestUnmarshal_InvalidInt(t *testing.T) {
	type R struct {
		Code string `ocr:"0:2"`
		Num  int    `ocr:"2:10"`
	}
	line := "ABABCDEFGH"
	var r R
	err := Unmarshal(line, &r)
	if err == nil {
		t.Fatal("expected error for non-numeric int field")
	}
}

// Invalid uint parse
func TestUnmarshal_InvalidUint(t *testing.T) {
	type R struct {
		Code  string `ocr:"0:2"`
		Count uint   `ocr:"2:10"`
	}
	line := "ABABCDEFGH"
	var r R
	err := Unmarshal(line, &r)
	if err == nil {
		t.Fatal("expected error for non-numeric uint field")
	}
}

// parseTag unknown option
func TestParseTag_UnknownOption(t *testing.T) {
	type R struct {
		Code string `ocr:"0:2,bogus"`
	}
	_, err := Marshal(R{Code: "AB"})
	if err == nil {
		t.Fatal("expected error for unknown tag option")
	}
}

// InvalidMarshalError nil type (Marshal called with untyped nil)
func TestInvalidMarshalError_NilType(t *testing.T) {
	e := &InvalidMarshalError{Type: nil}
	if e.Error() != "ocrline: Marshal(nil)" {
		t.Errorf("Error() = %q", e.Error())
	}
}

// Filler interface via pointer receiver (pass pointer to Marshal)
type RecordWithPointerFiller struct {
	Code string `ocr:"0:2"`
}

func (r *RecordWithPointerFiller) OCRFill() []Fill {
	return []Fill{
		{Start: 2, End: 10, Char: ' '},
	}
}

func TestMarshal_FillerViaPointerReceiver(t *testing.T) {
	r := &RecordWithPointerFiller{Code: "AB"}
	line, err := MarshalWidth(r, 10)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if line[2:10] != "        " {
		t.Errorf("fill = %q, want 8 spaces", line[2:10])
	}
}

// Value-receiver Unmarshaler error path
type ValueFailUnmarshaler string

func (v ValueFailUnmarshaler) UnmarshalOCR(data string) error {
	return fmt.Errorf("value unmarshal failed")
}

type RecordWithValueFailUnmarshaler struct {
	Code string               `ocr:"0:2"`
	Val  ValueFailUnmarshaler `ocr:"2:10"`
}

func TestUnmarshal_ValueReceiverUnmarshalerError(t *testing.T) {
	line := "ABXXXXXXXX" + strings.Repeat("0", 70)
	var r RecordWithValueFailUnmarshaler
	err := Unmarshal(line, &r)
	if err == nil {
		t.Fatal("expected error from value-receiver unmarshaler")
	}
}

// parseTag: invalid end index
func TestParseTag_InvalidEnd(t *testing.T) {
	type R struct {
		Code string `ocr:"0:abc"`
	}
	_, err := Marshal(R{Code: "AB"})
	if err == nil {
		t.Fatal("expected error for invalid end index")
	}
}

// parseTag: invalid start index
func TestParseTag_InvalidStart(t *testing.T) {
	type R struct {
		Code string `ocr:"abc:10"`
	}
	_, err := Marshal(R{Code: "AB"})
	if err == nil {
		t.Fatal("expected error for invalid start index")
	}
}

// --- Benchmark ---

func BenchmarkMarshal(b *testing.B) {
	r := TransmissionStart{
		RecordBaseInfo: RecordBaseInfo{
			FormatCode:  "NY",
			ServiceCode: "00",
			Type:        "00",
			RecordType:  10,
		},
		DataTransmitter:    "55555555",
		TransmissionNumber: "1000081",
		DataRecipient:      "00008080",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Marshal(&r)
	}
}

func BenchmarkUnmarshal(b *testing.B) {
	line := "NY000010555555551000081000080800000000000000000000000000000000000000000000000000"
	var r TransmissionStart
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Unmarshal(line, &r)
	}
}
