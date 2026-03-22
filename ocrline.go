// Package ocrline provides Marshal and Unmarshal functions for fixed-width
// line-based file formats such as Nets AvtaleGiro, OCR Giro, and Bankgirot AutoGiro.
//
// Struct fields are annotated with `ocr` tags that specify their position
// within a fixed-width line, along with optional alignment and padding directives.
//
// # Tag Syntax
//
//	ocr:"start:end[,option...]"
//
// Where start and end are zero-based byte positions (like Go slice indices),
// and options can be:
//
//   - align-left, align-right  — field alignment (default depends on type)
//   - pad-zero, pad-space      — padding character (default depends on type)
//   - omitempty                — if the field is zero-valued, fill with padding instead
//
// Fields without an `ocr` tag are skipped. Embedded structs are traversed recursively.
//
// # Gaps Between Fields
//
// Byte positions not covered by any struct field are filled with '0' by default.
// To fill specific gaps with a different character (e.g. spaces), implement the
// [Filler] interface on the record struct:
//
//	func (r PaymentClaim) OCRFill() []ocrline.Fill {
//	    return []ocrline.Fill{
//	        {Start: 21, End: 32, Char: ' '},
//	    }
//	}
//
// # Type Defaults
//
// The library uses the Go type of each field to determine default alignment and padding:
//
//   - int, int8..int64, uint..uint64: right-aligned, zero-padded
//   - string: left-aligned, space-padded
//   - Types implementing [Marshaler] / [Unmarshaler]: delegated to the type
//
// # Custom Types
//
// Types can implement [Marshaler] and [Unmarshaler] to control their own
// serialization, similar to encoding/json:
//
//	type ServiceCode string
//
//	func (s ServiceCode) MarshalOCR() (string, error) { return string(s), nil }
//	func (s *ServiceCode) UnmarshalOCR(data string) error { *s = ServiceCode(data); return nil }
//
// # Validation
//
// Struct tag metadata is parsed, validated, and cached on first use of a type
// (like encoding/json). Subsequent calls for the same struct type incur no
// reflection overhead. The following are validated once per type:
//
//   - Tag syntax: start and end must be valid integers, start >= 0, end > start
//   - Overlapping fields: two fields covering the same byte positions are rejected
//
// On each Unmarshal call, field ranges are checked against the input line length.
//
// # Line Width
//
// By default, Marshal pads the output to 80 characters. Use [MarshalWidth] to
// specify a different line width, or pass 0 to disable padding.
//
// # Usage
//
//	type Header struct {
//	    FormatCode  string `ocr:"0:2"`
//	    ServiceCode string `ocr:"2:4"`
//	    RecordType  int    `ocr:"6:8"`
//	}
//
//	// Unmarshal
//	var h Header
//	err := ocrline.Unmarshal("NY000010...", &h)
//
//	// Marshal
//	line, err := ocrline.Marshal(h)
package ocrline

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// DefaultLineWidth is the default output line width used by Marshal.
const DefaultLineWidth = 80

// Marshaler is the interface implemented by types that can marshal themselves
// into a fixed-width OCR field string.
type Marshaler interface {
	MarshalOCR() (string, error)
}

// Unmarshaler is the interface implemented by types that can unmarshal
// a fixed-width OCR field string into themselves.
type Unmarshaler interface {
	UnmarshalOCR(data string) error
}

// Fill describes a range of bytes that should be filled with a specific
// character during marshalling. This is used to fill gaps between fields
// with characters other than the default '0'.
type Fill struct {
	Start int
	End   int
	Char  byte
}

// Filler is an optional interface that record structs can implement to
// specify how gaps (byte positions not covered by any field) should be
// filled during marshalling.
//
// Gaps not covered by any Fill entry default to '0'.
//
// Example:
//
//	func (r PaymentClaim) OCRFill() []ocrline.Fill {
//	    return []ocrline.Fill{
//	        {Start: 21, End: 32, Char: ' '},
//	    }
//	}
type Filler interface {
	OCRFill() []Fill
}

// --- Cached struct metadata ---

// fieldInfo holds pre-parsed metadata for a single struct field with an ocr tag.
type fieldInfo struct {
	index     []int // field index path (for nested embedded structs)
	name      string
	tag       tagOptions
	fieldType reflect.Type
	align     alignment
	pad       padding
}

// structInfo holds cached, validated metadata for a struct type.
type structInfo struct {
	fields []fieldInfo
}

// cachedResult holds either a successful structInfo or an error from validation.
type cachedResult struct {
	info *structInfo
	err  error
}

var structCache sync.Map // map[reflect.Type]cachedResult

// getStructInfo returns cached struct metadata, computing and validating it on first access.
func getStructInfo(t reflect.Type) (*structInfo, error) {
	if cached, ok := structCache.Load(t); ok {
		r := cached.(cachedResult)
		return r.info, r.err
	}

	r, _ := structCache.LoadOrStore(t, buildStructInfo(t))
	result := r.(cachedResult)
	return result.info, result.err
}

func buildStructInfo(t reflect.Type) cachedResult {
	var fields []fieldInfo
	if err := collectFields(t, nil, "", &fields); err != nil {
		return cachedResult{err: err}
	}

	// Sort by start position for overlap check
	sort.Slice(fields, func(i, j int) bool {
		if fields[i].tag.start != fields[j].tag.start {
			return fields[i].tag.start < fields[j].tag.start
		}
		return fields[i].tag.end < fields[j].tag.end
	})

	// Check for overlaps
	for i := 1; i < len(fields); i++ {
		prev := fields[i-1]
		curr := fields[i]
		if curr.tag.start < prev.tag.end {
			return cachedResult{err: &OverlapError{
				Field1: prev.name, Start1: prev.tag.start, End1: prev.tag.end,
				Field2: curr.name, Start2: curr.tag.start, End2: curr.tag.end,
			}}
		}
	}

	return cachedResult{info: &structInfo{fields: fields}}
}

// collectFields recursively collects field metadata from a struct type.
func collectFields(t reflect.Type, index []int, prefix string, fields *[]fieldInfo) error {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldIndex := append(append([]int(nil), index...), i)
		name := prefix + field.Name

		// Recurse into embedded/anonymous structs (value or pointer)
		if field.Anonymous {
			ft := field.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				if err := collectFields(ft, fieldIndex, name+".", fields); err != nil {
					return err
				}
				continue
			}
		}

		// Skip unexported non-embedded fields
		if !field.IsExported() {
			continue
		}

		tag := field.Tag.Get("ocr")
		if tag == "" {
			continue
		}

		opts, err := parseTag(tag)
		if err != nil {
			return &TagError{Field: name, Tag: tag, Err: err}
		}

		a := resolveAlignment(opts, field.Type)
		p := resolvePadding(opts, field.Type)

		*fields = append(*fields, fieldInfo{
			index:     fieldIndex,
			name:      name,
			tag:       opts,
			fieldType: field.Type,
			align:     a,
			pad:       p,
		})
	}
	return nil
}

// --- Public API ---

// Marshal returns the OCR line encoding of v, padded to DefaultLineWidth (80) characters.
//
// v must be a struct or a pointer to a struct. Fields are encoded according
// to their `ocr` tags. Any positions not covered by struct fields are filled
// with '0'.
//
// Marshal traverses embedded structs recursively.
func Marshal(v any) (string, error) {
	return MarshalWidth(v, DefaultLineWidth)
}

// MarshalWidth returns the OCR line encoding of v, padded to the specified width.
// If width is 0, no padding is applied and the line is exactly as wide as the
// rightmost field end position.
//
// v must be a struct or a pointer to a struct.
func MarshalWidth(v any, width int) (string, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return "", &InvalidMarshalError{reflect.TypeOf(v)}
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return "", &InvalidMarshalError{reflect.TypeOf(v)}
	}

	info, err := getStructInfo(rv.Type())
	if err != nil {
		return "", err
	}

	maxEnd := 0
	segments := make(map[int]string, len(info.fields))

	for _, fi := range info.fields {
		fieldVal := fieldByIndex(rv, fi.index)
		if !fieldVal.IsValid() {
			continue // nil embedded pointer
		}

		if fi.tag.end > maxEnd {
			maxEnd = fi.tag.end
		}

		w := fi.tag.end - fi.tag.start
		var str string

		// Check for Marshaler interface
		if m, ok := marshalerFor(fieldVal); ok {
			s, err := m.MarshalOCR()
			if err != nil {
				return "", &MarshalFieldError{Field: fi.name, Err: err}
			}
			str = s
		} else {
			str = fieldToString(fieldVal)
		}

		// Handle omitempty
		if fi.tag.omitempty && str == "" {
			str = strings.Repeat(string(fi.pad), w)
			segments[fi.tag.start] = str
			continue
		}

		segments[fi.tag.start] = padString(str, w, fi.pad, fi.align)
	}

	// Determine effective width
	effectiveWidth := width
	if effectiveWidth == 0 {
		effectiveWidth = maxEnd
	}

	// Build the output line
	buf := make([]byte, effectiveWidth)
	for i := range buf {
		buf[i] = '0' // default gap fill
	}

	// Apply custom gap fills if the struct implements Filler.
	if f, ok := v.(Filler); ok {
		for _, fill := range f.OCRFill() {
			end := min(fill.End, effectiveWidth)
			for i := fill.Start; i < end; i++ {
				buf[i] = fill.Char
			}
		}
	}

	for _, fi := range info.fields {
		if s, ok := segments[fi.tag.start]; ok {
			copy(buf[fi.tag.start:], s)
		}
	}

	return string(buf), nil
}

// Unmarshal parses an OCR line and stores the result in the value pointed to by v.
//
// v must be a pointer to a struct. Fields are decoded according to their `ocr` tags.
// Unmarshal traverses embedded structs recursively.
func Unmarshal(line string, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return &InvalidUnmarshalError{reflect.TypeOf(v)}
	}

	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return &InvalidUnmarshalError{reflect.TypeOf(v)}
	}

	info, err := getStructInfo(rv.Type())
	if err != nil {
		return err
	}

	for _, fi := range info.fields {
		// Check field fits within the input line
		if fi.tag.end > len(line) {
			return &UnmarshalRangeError{
				Field:     fi.name,
				Start:     fi.tag.start,
				End:       fi.tag.end,
				LineWidth: len(line),
			}
		}

		fieldVal := fieldByIndex(rv, fi.index)
		if !fieldVal.IsValid() || !fieldVal.CanSet() {
			return &UnmarshalFieldError{Field: fi.name, Err: fmt.Errorf("cannot set field (unexported?)")}
		}

		slice := line[fi.tag.start:fi.tag.end]

		// Check for Unmarshaler interface (pointer receiver)
		if fieldVal.CanAddr() {
			if u, ok := fieldVal.Addr().Interface().(Unmarshaler); ok {
				if err := u.UnmarshalOCR(slice); err != nil {
					return &UnmarshalFieldError{Field: fi.name, Err: err}
				}
				continue
			}
		}
		// Check value receiver
		if fieldVal.CanInterface() {
			if u, ok := fieldVal.Interface().(Unmarshaler); ok {
				if err := u.UnmarshalOCR(slice); err != nil {
					return &UnmarshalFieldError{Field: fi.name, Err: err}
				}
				continue
			}
		}

		if err := setField(fieldVal, slice); err != nil {
			return &UnmarshalFieldError{Field: fi.name, Err: err}
		}
	}

	return nil
}

// --- Internal helpers ---

// fieldByIndex traverses a struct value by field index path, allocating
// nil embedded pointers along the way (for unmarshal) or returning
// invalid Value if nil (for marshal).
func fieldByIndex(rv reflect.Value, index []int) reflect.Value {
	for _, i := range index {
		if rv.Kind() == reflect.Pointer {
			if rv.IsNil() {
				// For unmarshal: allocate. For marshal: caller checks IsValid.
				if rv.CanSet() {
					rv.Set(reflect.New(rv.Type().Elem()))
				} else {
					return reflect.Value{}
				}
			}
			rv = rv.Elem()
		}
		rv = rv.Field(i)
	}
	return rv
}

func marshalerFor(v reflect.Value) (Marshaler, bool) {
	if v.CanInterface() {
		if m, ok := v.Interface().(Marshaler); ok {
			return m, true
		}
	}
	if v.CanAddr() {
		if m, ok := v.Addr().Interface().(Marshaler); ok {
			return m, true
		}
	}
	return nil, false
}

func fieldToString(v reflect.Value) string {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10)
	case reflect.String:
		return v.String()
	case reflect.Bool:
		if v.Bool() {
			return "1"
		}
		return "0"
	case reflect.Pointer:
		if v.IsNil() {
			return ""
		}
		return fieldToString(v.Elem())
	default:
		return fmt.Sprintf("%v", v.Interface())
	}
}

func setField(v reflect.Value, data string) error {
	switch v.Kind() {
	case reflect.String:
		v.SetString(strings.TrimSpace(data))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		trimmed := strings.TrimSpace(data)
		if trimmed == "" {
			v.SetInt(0)
			return nil
		}
		n, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil {
			return fmt.Errorf("cannot parse %q as int: %w", data, err)
		}
		v.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		trimmed := strings.TrimSpace(data)
		if trimmed == "" {
			v.SetUint(0)
			return nil
		}
		n, err := strconv.ParseUint(trimmed, 10, 64)
		if err != nil {
			return fmt.Errorf("cannot parse %q as uint: %w", data, err)
		}
		v.SetUint(n)
	case reflect.Bool:
		trimmed := strings.TrimSpace(data)
		switch trimmed {
		case "1", "J", "Y", "true":
			v.SetBool(true)
		case "0", "N", "false", "":
			v.SetBool(false)
		default:
			return fmt.Errorf("cannot parse %q as bool", data)
		}
	case reflect.Pointer:
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		return setField(v.Elem(), data)
	default:
		return fmt.Errorf("unsupported field type: %v", v.Kind())
	}
	return nil
}

// --- Internal: Tag parsing ---

type tagOptions struct {
	start     int
	end       int
	align     alignment
	pad       padding
	omitempty bool
}

type alignment int

const (
	alignDefault alignment = iota
	alignLeft
	alignRight
)

type padding byte

const (
	padDefault padding = 0
	padZero    padding = '0'
	padSpace   padding = ' '
)

func parseTag(tag string) (tagOptions, error) {
	parts := strings.Split(tag, ",")
	if len(parts) == 0 {
		return tagOptions{}, fmt.Errorf("empty tag")
	}

	indices := strings.Split(parts[0], ":")
	if len(indices) != 2 {
		return tagOptions{}, fmt.Errorf("expected 'start:end', got %q", parts[0])
	}

	start, err := strconv.Atoi(strings.TrimSpace(indices[0]))
	if err != nil {
		return tagOptions{}, fmt.Errorf("invalid start index %q: %w", indices[0], err)
	}
	end, err := strconv.Atoi(strings.TrimSpace(indices[1]))
	if err != nil {
		return tagOptions{}, fmt.Errorf("invalid end index %q: %w", indices[1], err)
	}

	if start < 0 {
		return tagOptions{}, fmt.Errorf("start must be >= 0, got %d", start)
	}
	if end <= start {
		return tagOptions{}, fmt.Errorf("end (%d) must be > start (%d)", end, start)
	}

	opts := tagOptions{start: start, end: end}

	for _, part := range parts[1:] {
		part = strings.TrimSpace(part)
		switch part {
		case "align-left":
			opts.align = alignLeft
		case "align-right":
			opts.align = alignRight
		case "pad-zero":
			opts.pad = padZero
		case "pad-space":
			opts.pad = padSpace
		case "omitempty":
			opts.omitempty = true
		default:
			return tagOptions{}, fmt.Errorf("unknown option %q", part)
		}
	}

	return opts, nil
}

func resolveAlignment(opts tagOptions, t reflect.Type) alignment {
	if opts.align != alignDefault {
		return opts.align
	}
	switch underlyingKind(t) {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return alignRight
	case reflect.String:
		return alignLeft
	default:
		return alignRight
	}
}

func resolvePadding(opts tagOptions, t reflect.Type) padding {
	if opts.pad != padDefault {
		return opts.pad
	}
	switch underlyingKind(t) {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return padZero
	case reflect.String:
		return padSpace
	default:
		return padZero
	}
}

func underlyingKind(t reflect.Type) reflect.Kind {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Kind()
}

func padString(s string, width int, pad padding, align alignment) string {
	if len(s) >= width {
		return s[:width]
	}
	fill := strings.Repeat(string(pad), width-len(s))
	if align == alignLeft {
		return s + fill
	}
	return fill + s
}
