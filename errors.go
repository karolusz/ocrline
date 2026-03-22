package ocrline

import (
	"fmt"
	"reflect"
)

// OverlapError describes two fields whose ocr tag ranges overlap.
type OverlapError struct {
	Field1       string
	Start1, End1 int
	Field2       string
	Start2, End2 int
}

func (e *OverlapError) Error() string {
	return fmt.Sprintf(
		"ocrline: fields %s [%d:%d] and %s [%d:%d] overlap",
		e.Field1, e.Start1, e.End1,
		e.Field2, e.Start2, e.End2,
	)
}

// InvalidMarshalError describes an invalid argument passed to Marshal.
type InvalidMarshalError struct {
	Type reflect.Type
}

func (e *InvalidMarshalError) Error() string {
	if e.Type == nil {
		return "ocrline: Marshal(nil)"
	}
	if e.Type.Kind() == reflect.Pointer {
		return "ocrline: Marshal(nil " + e.Type.String() + ")"
	}
	return "ocrline: Marshal(non-struct " + e.Type.String() + ")"
}

// InvalidUnmarshalError describes an invalid argument passed to Unmarshal.
type InvalidUnmarshalError struct {
	Type reflect.Type
}

func (e *InvalidUnmarshalError) Error() string {
	if e.Type == nil {
		return "ocrline: Unmarshal(nil)"
	}
	if e.Type.Kind() != reflect.Pointer {
		return "ocrline: Unmarshal(non-pointer " + e.Type.String() + ")"
	}
	return "ocrline: Unmarshal(nil " + e.Type.String() + ")"
}

// TagError describes an error in an ocr struct tag.
type TagError struct {
	Field string
	Tag   string
	Err   error
}

func (e *TagError) Error() string {
	return fmt.Sprintf("ocrline: invalid tag on field %s: %q: %v", e.Field, e.Tag, e.Err)
}

func (e *TagError) Unwrap() error {
	return e.Err
}

// MarshalFieldError describes an error marshalling a specific field.
type MarshalFieldError struct {
	Field string
	Err   error
}

func (e *MarshalFieldError) Error() string {
	return fmt.Sprintf("ocrline: error marshalling field %s: %v", e.Field, e.Err)
}

func (e *MarshalFieldError) Unwrap() error {
	return e.Err
}

// UnmarshalFieldError describes an error unmarshalling a specific field.
type UnmarshalFieldError struct {
	Field string
	Err   error
}

func (e *UnmarshalFieldError) Error() string {
	return fmt.Sprintf("ocrline: error unmarshalling field %s: %v", e.Field, e.Err)
}

func (e *UnmarshalFieldError) Unwrap() error {
	return e.Err
}

// UnmarshalRangeError describes a field whose ocr tag range exceeds the input line.
type UnmarshalRangeError struct {
	Field     string
	Start     int
	End       int
	LineWidth int
}

func (e *UnmarshalRangeError) Error() string {
	return fmt.Sprintf(
		"ocrline: field %s range [%d:%d] exceeds line width %d",
		e.Field, e.Start, e.End, e.LineWidth,
	)
}
