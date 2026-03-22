# ocrline

[![CI](https://github.com/karolusz/ocrline/actions/workflows/ci.yml/badge.svg)](https://github.com/karolusz/ocrline/actions/workflows/ci.yml)
[![coverage](https://raw.githubusercontent.com/karolusz/ocrline/badges/coverage-badge.svg)](https://github.com/karolusz/ocrline/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/karolusz/ocrline.svg)](https://pkg.go.dev/github.com/karolusz/ocrline)
[![Go Report Card](https://goreportcard.com/badge/github.com/karolusz/ocrline)](https://goreportcard.com/report/github.com/karolusz/ocrline)

Marshal and unmarshal Go structs to and from fixed-width line formats using struct tags.

Built for Scandinavian payment file formats (Nets AvtaleGiro, OCR Giro, Bankgirot AutoGiro) but works with any 80-character (or custom width) fixed-position record format.

## Install

```
go get github.com/karolusz/ocrline
```

## Usage

Define structs with `ocr` tags specifying 0-based byte positions (Go slice convention):

```go
type TransmissionStart struct {
    FormatCode      string `ocr:"0:2"`
    ServiceCode     string `ocr:"2:4"`
    TransactionType string `ocr:"4:6"`
    RecordType      int    `ocr:"6:8"`
    DataTransmitter string `ocr:"8:16"`
    TransmissionNo  string `ocr:"16:23"`
    DataRecipient   string `ocr:"23:31"`
}
```

### Unmarshal

```go
line := "NY000010555555551000081000080800000000000000000000000000000000000000000000000000"

var record TransmissionStart
if err := ocrline.Unmarshal(line, &record); err != nil {
    log.Fatal(err)
}

fmt.Println(record.FormatCode)      // "NY"
fmt.Println(record.DataTransmitter) // "55555555"
fmt.Println(record.RecordType)      // 10
```

### Marshal

```go
record := TransmissionStart{
    FormatCode:      "NY",
    ServiceCode:     "00",
    TransactionType: "00",
    RecordType:      10,
    DataTransmitter: "55555555",
    TransmissionNo:  "1000081",
    DataRecipient:   "00008080",
}

line, err := ocrline.Marshal(record)
// "NY000010555555551000081000080800000000000000000000000000000000000000000000000000"
```

## Tag Syntax

```
ocr:"start:end"
ocr:"start:end,option,option,..."
```

Positions are **0-based, exclusive end** (like Go slices). `ocr:"0:2"` reads `line[0:2]`.

### Options

| Option | Description |
|--------|-------------|
| `align-left` | Left-align the value in the field |
| `align-right` | Right-align the value in the field |
| `pad-zero` | Pad with `'0'` characters |
| `pad-space` | Pad with `' '` characters |
| `omitempty` | If zero-valued, fill with padding instead of the value |

### Type Defaults

| Go Type | Default Alignment | Default Padding |
|---------|-------------------|-----------------|
| `string` | left | space |
| `int`, `int8`..`int64` | right | zero |
| `uint`, `uint8`..`uint64` | right | zero |
| `bool` | right | zero |
| `*T` | inherits from `T` | inherits from `T` |

Named types follow their underlying type: `type Numeric string` gets string defaults.

## Struct Composition

Embedded structs are flattened, just like `encoding/json`:

```go
type RecordBase struct {
    FormatCode  string `ocr:"0:2"`
    ServiceCode string `ocr:"2:4"`
    RecordType  int    `ocr:"6:8"`
}

type PaymentRecord struct {
    RecordBase
    PayerNumber string `ocr:"15:31"`
    Amount      int    `ocr:"31:43"`
}
```

Embedded pointer structs (`*RecordBase`) are also supported. On unmarshal, nil pointers are auto-allocated. On marshal, nil embedded pointers are skipped (gaps filled with default).

## Gap Filling

Byte positions not covered by any field are filled with `'0'` by default. To fill specific gaps with a different character, implement the `Filler` interface:

```go
func (r PaymentRecord) OCRFill() []ocrline.Fill {
    return []ocrline.Fill{
        {Start: 8, End: 15, Char: ' '},  // positions 8-14 filled with spaces
    }
}
```

## Custom Types

Implement `Marshaler` and `Unmarshaler` for full control over field serialization:

```go
type ServiceCode string

func (s ServiceCode) MarshalOCR() (string, error) {
    return string(s), nil
}

func (s *ServiceCode) UnmarshalOCR(data string) error {
    *s = ServiceCode(strings.TrimSpace(data))
    return nil
}
```

## Line Width

`Marshal` outputs 80 characters by default. Use `MarshalWidth` for other widths:

```go
line, err := ocrline.MarshalWidth(record, 120)  // 120-char line
line, err := ocrline.MarshalWidth(record, 0)    // no padding, exact width of rightmost field
```

## Validation and Caching

Struct metadata is parsed, validated, and cached per type on first use (same pattern as `encoding/json`). Subsequent calls for the same struct type have zero reflection overhead for tag parsing.

Validated once per type:

- **Tag syntax** - start and end must be integers, `start >= 0`, `end > start`
- **Overlapping fields** - two fields covering the same positions are rejected with `*OverlapError`
- **Invalid tags** - malformed `ocr` tags produce `*TagError`

Validated per call:

- **Out-of-range fields** - on unmarshal, fields exceeding the line length produce `*UnmarshalRangeError`
- **Unexported fields** are silently skipped (same as `encoding/json`)

## API

```go
func Marshal(v any) (string, error)
func MarshalWidth(v any, width int) (string, error)
func Unmarshal(line string, v any) error
```

### Interfaces

```go
type Marshaler interface {
    MarshalOCR() (string, error)
}

type Unmarshaler interface {
    UnmarshalOCR(data string) error
}

type Filler interface {
    OCRFill() []Fill
}
```

## Supported Formats

The library is format-agnostic. It has been designed with these formats in mind:

| Format | Country | Provider | Line Width |
|--------|---------|----------|------------|
| AvtaleGiro | Norway | Nets | 80 |
| OCR Giro | Norway | Nets | 80 |
| AutoGiro | Sweden | Bankgirot | 80 |

## License

MIT
