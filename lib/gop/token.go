package gop

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"time"
	"unicode"
	"unicode/utf8"
)

// LongStringLen is the length of that will be treated as long string
var LongStringLen = 16

// LongBytesLen is the length of that will be treated as long bytes
var LongBytesLen = 16

// Type of token
type Type int

const (
	// Nil type
	Nil Type = iota
	// Bool type
	Bool
	// Number type
	Number
	// Float type
	Float
	// Complex type
	Complex
	// String type
	String
	// Byte type
	Byte
	// Rune type
	Rune
	// Chan type
	Chan
	// Func type
	Func
	// Error type
	Error

	// Comment type
	Comment

	// TypeName type
	TypeName

	// ParenOpen type
	ParenOpen
	// ParenClose type
	ParenClose

	// Dot type
	Dot
	// And type
	And

	// SliceOpen type
	SliceOpen
	// SliceItem type
	SliceItem
	// InlineComma type
	InlineComma
	// Comma type
	Comma
	// SliceClose type
	SliceClose

	// MapOpen type
	MapOpen
	// MapKey type
	MapKey
	// Colon type
	Colon
	// MapClose type
	MapClose

	// StructOpen type
	StructOpen
	// StructKey type
	StructKey
	// StructField type
	StructField
	// StructClose type
	StructClose
)

// Token represents a symbol in value layout
type Token struct {
	Type    Type
	Literal string
}

// Tokenize a random Go value
func Tokenize(v interface{}) []*Token {
	return tokenize(seen{}, []interface{}{}, reflect.ValueOf(v))
}

// Any type
type Any interface{}

// Obj type
type Obj map[string]Any

// Arr type
type Arr []Any

// Ptr returns a pointer to v
func Ptr(v interface{}) interface{} {
	val := reflect.ValueOf(v)
	ptr := reflect.New(val.Type())
	ptr.Elem().Set(val)
	return ptr.Interface()
}

// Circular reference of the path from the root
func Circular(path ...interface{}) interface{} {
	return nil
}

// Base64 returns the []byte that s represents
func Base64(s string) []byte {
	b, _ := base64.StdEncoding.DecodeString(s)
	return b
}

// Time from parsing s
func Time(s string, monotonic int) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

// Duration from parsing s
func Duration(s string) time.Duration {
	d, _ := time.ParseDuration(s)
	return d
}

// JSONStr returns the raw
func JSONStr(v interface{}, raw string) string {
	return raw
}

// JSONBytes returns the raw as []byte
func JSONBytes(v interface{}, raw string) []byte {
	return []byte(raw)
}

type path []interface{}

func (p path) tokens() []*Token {
	sn := map[uintptr]path{}
	ts := []*Token{}
	for i, seg := range p {
		ts = append(ts, tokenize(sn, []interface{}{}, reflect.ValueOf(seg))...)
		if i < len(p)-1 {
			ts = append(ts, &Token{InlineComma, ","})
		}
	}
	return ts
}

type seen map[uintptr]path

func (sn seen) circular(p path, v reflect.Value) []*Token {
	switch v.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice:
		ptr := v.Pointer()
		if p, has := sn[ptr]; has {
			ts := []*Token{{Func, "gop.Circular"}, {ParenOpen, "("}}
			ts = append(ts, p.tokens()...)
			return append(ts, &Token{ParenClose, ")"}, &Token{Dot, "."},
				&Token{ParenOpen, "("}, typeName(v.Type().String()), &Token{ParenClose, ")"})
		}
		sn[ptr] = p
	}

	return nil
}

func tokenize(sn seen, p path, v reflect.Value) []*Token {
	if ts, has := tokenizeSpecial(v); has {
		return ts
	}

	if ts := sn.circular(p, v); ts != nil {
		return ts
	}

	t := &Token{Nil, ""}

	switch v.Kind() {
	case reflect.Interface:
		return tokenize(sn, p, v.Elem())

	case reflect.Bool:
		t.Type = Bool
		if v.Bool() {
			t.Literal = "true"
		} else {
			t.Literal = "false"
		}

	case reflect.String:
		return tokenizeString(v)

	case reflect.Chan:
		if v.Cap() == 0 {
			return []*Token{{Func, "make"}, {ParenOpen, "("},
				{Chan, "chan"}, typeName(v.Type().Elem().String()), {ParenClose, ")"},
				{Comment, fmt.Sprintf("/* 0x%x */", v.Pointer())}}
		}
		return []*Token{{Func, "make"}, {ParenOpen, "("}, {Chan, "chan"},
			typeName(v.Type().Elem().Name()), {InlineComma, ","},
			{Number, fmt.Sprintf("%d", v.Cap())}, {ParenClose, ")"},
			{Comment, fmt.Sprintf("/* 0x%x */", v.Pointer())}}

	case reflect.Func:
		return []*Token{{ParenOpen, "("}, {TypeName, v.Type().String()},
			{ParenClose, ")"}, {ParenOpen, "("}, {Nil, "nil"}, {ParenClose, ")"},
			{Comment, fmt.Sprintf("/* 0x%x */", v.Pointer())}}

	case reflect.Ptr:
		return tokenizePtr(sn, p, v)

	case reflect.UnsafePointer:
		return []*Token{typeName("unsafe.Pointer"), {ParenOpen, "("}, typeName("uintptr"),
			{ParenOpen, "("}, typeName(fmt.Sprintf("%v", v.Interface())), {ParenClose, ")"}, {ParenClose, ")"}}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64,
		reflect.Uintptr, reflect.Complex64, reflect.Complex128:
		return tokenizeNumber(v)

	case reflect.Slice, reflect.Array, reflect.Map, reflect.Struct:
		return tokenizeCollection(sn, p, v)
	}

	return []*Token{t}
}

func tokenizeSpecial(v reflect.Value) ([]*Token, bool) {
	if v.Kind() == reflect.Invalid {
		return []*Token{{Nil, "nil"}}, true
	} else if r, ok := v.Interface().(rune); ok && unicode.IsGraphic(r) {
		return []*Token{tokenizeRune(&Token{Nil, ""}, r)}, true
	} else if b, ok := v.Interface().(byte); ok {
		return tokenizeByte(&Token{Nil, ""}, b), true
	} else if t, ok := v.Interface().(time.Time); ok {
		return tokenizeTime(t), true
	} else if d, ok := v.Interface().(time.Duration); ok {
		return tokenizeDuration(d), true
	}

	return tokenizeJSON(v)
}

func tokenizeCollection(sn seen, p path, v reflect.Value) []*Token {
	ts := []*Token{}

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		if data, ok := v.Interface().([]byte); ok {
			ts = append(ts, tokenizeBytes(data)...)
			break
		} else {
			ts = append(ts, typeName(v.Type().String()))
		}
		if v.Kind() == reflect.Slice {
			ts = append(ts, &Token{Comment, fmt.Sprintf("/* len=%d cap=%d */", v.Len(), v.Cap())})
		}
		ts = append(ts, &Token{SliceOpen, "{"})
		for i := 0; i < v.Len(); i++ {
			p := append(p, i)
			el := v.Index(i)
			ts = append(ts, &Token{SliceItem, ""})
			ts = append(ts, tokenize(sn, p, el)...)
			ts = append(ts, &Token{Comma, ","})
		}
		ts = append(ts, &Token{SliceClose, "}"})

	case reflect.Map:
		ts = append(ts, typeName(v.Type().String()))
		keys := v.MapKeys()
		sort.Slice(keys, func(i, j int) bool {
			return compare(keys[i].Interface(), keys[j].Interface()) < 0
		})
		if len(keys) > 1 {
			ts = append(ts, &Token{Comment, fmt.Sprintf("/* len=%d */", len(keys))})
		}
		ts = append(ts, &Token{MapOpen, "{"})
		for _, k := range keys {
			p := append(p, k.Interface())
			ts = append(ts, &Token{MapKey, ""})
			ts = append(ts, tokenize(sn, p, k)...)
			ts = append(ts, &Token{Colon, ":"})
			ts = append(ts, tokenize(sn, p, v.MapIndex(k))...)
			ts = append(ts, &Token{Comma, ","})
		}
		ts = append(ts, &Token{MapClose, "}"})

	case reflect.Struct:
		t := v.Type()

		ts = append(ts, typeName(t.String()))
		if v.NumField() > 1 {
			ts = append(ts, &Token{Comment, fmt.Sprintf("/* len=%d */", v.NumField())})
		}
		ts = append(ts, &Token{StructOpen, "{"})
		for i := 0; i < v.NumField(); i++ {
			name := t.Field(i).Name
			ts = append(ts, &Token{StructKey, ""})
			ts = append(ts, &Token{StructField, name})

			f := v.Field(i)
			if !f.CanInterface() {
				f = GetPrivateField(v, i)
			}
			ts = append(ts, &Token{Colon, ":"})
			ts = append(ts, tokenize(sn, append(p, name), f)...)
			ts = append(ts, &Token{Comma, ","})
		}
		ts = append(ts, &Token{StructClose, "}"})
	}

	return ts
}

func tokenizeNumber(v reflect.Value) []*Token {
	t := &Token{Nil, ""}
	ts := []*Token{}

	switch v.Kind() {
	case reflect.Int:
		t.Type = Number
		t.Literal = strconv.FormatInt(v.Int(), 10)
		ts = append(ts, t)

	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64,
		reflect.Uintptr:

		ts = append(ts, typeName(v.Type().Name()), &Token{ParenOpen, "("})
		t.Type = Number
		t.Literal = fmt.Sprintf("%v", v.Interface())
		ts = append(ts, t, &Token{ParenClose, ")"})

	case reflect.Complex64:
		ts = append(ts, typeName(v.Type().Name()), &Token{ParenOpen, "("})
		t.Type = Number
		t.Literal = fmt.Sprintf("%v", v.Interface())
		t.Literal = t.Literal[1 : len(t.Literal)-1]
		ts = append(ts, t, &Token{ParenClose, ")"})

	case reflect.Complex128:
		t.Type = Number
		t.Literal = fmt.Sprintf("%v", v.Interface())
		t.Literal = t.Literal[1 : len(t.Literal)-1]
		ts = append(ts, t)
	}

	return ts
}

func tokenizeRune(t *Token, r rune) *Token {
	t.Type = Rune
	t.Literal = fmt.Sprintf("'%s'", string(r))
	return t
}

func tokenizeByte(t *Token, b byte) []*Token {
	ts := []*Token{typeName("byte"), {ParenOpen, "("}}
	if unicode.IsGraphic(rune(b)) {
		ts = append(ts, &Token{Byte, fmt.Sprintf("'%s'", string(b))})
	} else {
		ts = append(ts, &Token{Byte, fmt.Sprintf("0x%x", b)})
	}
	return append(ts, &Token{ParenClose, ")"})
}

func tokenizeTime(t time.Time) []*Token {
	ext := GetPrivateFieldByName(reflect.ValueOf(t), "ext").Int()
	ts := []*Token{{Func, "gop.Time"}, {ParenOpen, "("}}
	ts = append(ts, &Token{String, t.Format(time.RFC3339Nano)})
	ts = append(ts, &Token{InlineComma, ","}, &Token{Number, fmt.Sprintf("%d", ext)}, &Token{ParenClose, ")"})
	return ts
}

func tokenizeDuration(d time.Duration) []*Token {
	ts := []*Token{}
	ts = append(ts, typeName("gop.Duration"), &Token{ParenOpen, "("})
	ts = append(ts, &Token{String, d.String()})
	ts = append(ts, &Token{ParenClose, ")"})
	return ts
}

func tokenizeString(v reflect.Value) []*Token {
	s := v.String()
	ts := []*Token{{String, s}}
	if v.Len() >= LongStringLen {
		ts = append(ts, &Token{Comment, fmt.Sprintf("/* len=%d */", len(s))})
	}
	return ts
}

func tokenizeBytes(data []byte) []*Token {
	ts := []*Token{}

	if utf8.Valid(data) {
		s := string(data)
		ts = append(ts, typeName("[]byte"), &Token{ParenOpen, "("})
		ts = append(ts, &Token{String, s})
		ts = append(ts, &Token{ParenClose, ")"})
	} else {
		ts = append(ts, &Token{Func, "gop.Base64"}, &Token{ParenOpen, "("})
		ts = append(ts, &Token{String, base64.StdEncoding.EncodeToString(data)})
		ts = append(ts, &Token{ParenClose, ")"})
	}
	if len(data) >= LongBytesLen {
		ts = append(ts, &Token{Comment, fmt.Sprintf("/* len=%d */", len(data))})
	}
	return ts
}

func tokenizePtr(sn seen, p path, v reflect.Value) []*Token {
	ts := []*Token{}

	if v.Elem().Kind() == reflect.Invalid {
		ts = append(ts,
			&Token{ParenOpen, "("}, typeName(v.Type().String()), &Token{ParenClose, ")"},
			&Token{ParenOpen, "("}, &Token{Nil, "nil"}, &Token{ParenClose, ")"})
		return ts
	}

	fn := false

	switch v.Elem().Kind() {
	case reflect.Struct, reflect.Map, reflect.Slice, reflect.Array:
		if _, ok := v.Elem().Interface().([]byte); ok {
			fn = true
		}
	default:
		fn = true
	}

	if fn {
		ts = append(ts, &Token{Func, "gop.Ptr"}, &Token{ParenOpen, "("})
		ts = append(ts, tokenize(sn, p, v.Elem())...)
		ts = append(ts, &Token{ParenClose, ")"}, &Token{Dot, "."}, &Token{ParenOpen, "("},
			typeName(v.Type().String()), &Token{ParenClose, ")"})
	} else {
		ts = append(ts, &Token{And, "&"})
		ts = append(ts, tokenize(sn, p, v.Elem())...)
	}

	return ts
}

func tokenizeJSON(v reflect.Value) ([]*Token, bool) {
	var jv interface{}
	ts := []*Token{}
	s := ""
	if v.Kind() == reflect.String {
		s = v.String()
		err := json.Unmarshal([]byte(s), &jv)
		if err != nil {
			return nil, false
		}
		ts = append(ts, &Token{Func, "gop.JSONStr"})
	} else if b, ok := v.Interface().([]byte); ok {
		err := json.Unmarshal(b, &jv)
		if err != nil {
			return nil, false
		}
		s = string(b)
		ts = append(ts, &Token{Func, "gop.JSONBytes"})
	}

	_, isObj := jv.(map[string]interface{})
	_, isArr := jv.(map[string]interface{})

	if isObj || isArr {
		ts = append(ts, &Token{ParenOpen, "("})
		ts = append(ts, Tokenize(jv)...)
		ts = append(ts, &Token{InlineComma, ","},
			&Token{String, s}, &Token{ParenClose, ")"})
		return ts, true
	}

	return nil, false
}

func typeName(t string) *Token {
	switch t {
	case "map[string]interface {}":
		return &Token{TypeName, "gop.Obj"}
	case "[]interface {}":
		return &Token{TypeName, "gop.Arr"}
	default:
		return &Token{TypeName, t}
	}
}
