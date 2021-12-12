// Print a Go value as Go source.

// TODO:
// - testing complex, with and w/o elision
// - imports
// - ptr cycle detection
// - NaN map keys?
// - anonymous struct types?
// - testing printPtr with various imputed/elide combos
// - test 2-element inline maps
package main

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"reflect"
	"sort"
)

type Printer struct {
	pkgPath        string
	customPrinters map[reflect.Type]PrintFunc
	imports        map[string]string // from package path to identifier
}

func NewPrinter(packagePath string) *Printer {
	return &Printer{
		pkgPath:        packagePath,
		customPrinters: map[reflect.Type]PrintFunc{},
		imports:        map[string]string{},
	}
}

func (p *Printer) RegisterImport(packagePath, ident string) {
	p.imports[packagePath] = ident
}

type PrintFunc func(interface{}) (string, error)

func (p *Printer) RegisterCustomPrinter(valueForType interface{}, f PrintFunc) {
	p.customPrinters[reflect.TypeOf(valueForType)] = f
}

func (p *Printer) DetectCycles() {
	panic("unimp")
}

func (p *Printer) Sprint(value interface{}) (string, error) {
	var buf bytes.Buffer
	if err := p.Fprint(&buf, value); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (p *Printer) Fprint(w io.Writer, value interface{}) error {
	s := &state{
		w:   w,
		p:   p,
		err: nil,
	}
	s.print(value)
	return s.err
}

type state struct {
	p   *Printer
	w   io.Writer
	err error
}

func (s *state) print(value interface{}) {
	s.rprint(reflect.ValueOf(value), nil, false)
}

func (s *state) rprint(rv reflect.Value, imputedType reflect.Type, elide bool) {
	if !rv.IsValid() {
		s.printString("nil") // TODO: may need a cast
		return
	}
	if cp := s.p.customPrinters[rv.Type()]; cp != nil {
		out, err := cp(rv.Interface())
		if err != nil {
			s.err = err
			return
		}
		s.printString(out)
		return
	}

	switch rv.Kind() {
	case reflect.Bool, reflect.String, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		s.printPrimitiveLiteral(rv, imputedType)
	case reflect.Float32, reflect.Float64:
		s.printFloat(rv, imputedType)
	case reflect.Complex64, reflect.Complex128:
		s.printComplex(rv, imputedType)
	case reflect.Ptr:
		s.printPtr(rv, imputedType, elide)
	case reflect.Interface:
		s.rprint(rv.Elem(), imputedType, elide)
	case reflect.Slice, reflect.Array:
		s.printSliceOrArray(rv, imputedType, elide)
	case reflect.Map:
		s.printMap(rv, imputedType, elide)
	case reflect.Struct:
		s.printStruct(rv, imputedType, elide)
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		s.err = fmt.Errorf("cannot print %#v as source", rv.Interface())
	default:
		panic("bad kind")
	}
}

func (s *state) rsprint(rv reflect.Value, imputedType reflect.Type, elide bool) string {
	s2 := *s
	var buf bytes.Buffer
	s2.w = &buf
	s2.rprint(rv, imputedType, elide)
	return buf.String()
}

var (
	tBool       = reflect.TypeOf(false)
	tString     = reflect.TypeOf("")
	tInt        = reflect.TypeOf(int(0))
	tFloat64    = reflect.TypeOf(float64(0))
	tComplex128 = reflect.TypeOf(complex128(0))
)

// If the constant occurs in a context where it won't be converted
// to the right float type, we need an explicit conversion.
// This floating-point value could have been in a location whose underlying type
// is a float32 or float64, or whose underlying type is interface. In the first
// case no conversion is needed; in the second, one is.

func (s *state) printPrimitiveLiteral(v reflect.Value, imputedType reflect.Type) {

	vs := fmt.Sprintf("%#v", v.Interface()) // TODO: just v???  or v.String()??
	if v.Type() != defaultType(v) && (imputedType == nil || imputedType.Kind() == reflect.Interface) {
		s.printf("%s(%s)", s.sprintType(v.Type()), vs)
	} else {
		s.printString(vs)
	}
}

// func (s *state) printInt(v reflect.Value, imputedType reflect.Type) {
// 	vs := fmt.Sprintf("%#v", v.Interface())
// 	if v.Type() != tInt && !isNumeric(imputedType) {
// 		s.printf("%s(%s)", s.sprintType(v.Type()), vs)
// 	} else {
// 		s.printString(vs)
// 	}
// }

func (s *state) printFloat(v reflect.Value, imputedType reflect.Type) {
	f := v.Float()
	var str string
	switch {
	case math.IsNaN(f):
		str = "math.NaN()"
	case math.IsInf(f, 1):
		str = "math.Inf(1)"
	case math.IsInf(f, -1):
		str = "math.Inf(-1)"
	default:
		s.printPrimitiveLiteral(v, imputedType)
		return
	}
	if v.Type() == tFloat64 {
		s.printString(str)
	} else {
		// We can't omit the conversion here, regardless of the imputed type, because
		// this is not a constant literal.
		s.printf("%s(%s)", s.sprintType(v.Type()), str)
	}
}

func (s *state) printComplex(v reflect.Value, imputedType reflect.Type) {
	c := v.Complex()
	s.printf("complex(%s, %s)",
		s.rsprint(reflect.ValueOf(real(c)), nil, false),
		s.rsprint(reflect.ValueOf(imag(c)), nil, false))
}

func (s *state) printPtr(v reflect.Value, imputedType reflect.Type, elide bool) {
	elem := v.Elem()
	if s.printIfNil(v, imputedType) {
		return
	}
	if isPrimitive(elem.Kind()) {
		s.printf("func() *%s { var x %[1]s = %s; return &x }()",
			s.sprintType(elem.Type()), s.rsprint(elem, elem.Type(), false))
	} else if v.Type() == imputedType {
		s.rprint(elem, imputedType.Elem(), elide)
	} else {

		s.printString("&")
		s.rprint(elem, nil, false)
	}
}

func (s *state) printSliceOrArray(v reflect.Value, imputedType reflect.Type, elide bool) {
	if s.printIfNil(v, imputedType) {
		return
	}
	t := v.Type()
	var ts string
	if elide && t == imputedType {
		ts = ""
	} else {
		ts = s.sprintType(t)
	}

	if t.Kind() == reflect.Slice && v.IsNil() {
		if ts == "" {
			s.printString("nil")
		} else {
			s.printf("%s(nil)", ts)
		}
		return
	}

	s.printf("%s{", ts)

	k := t.Elem().Kind()
	if k != reflect.String && isPrimitive(k) && v.Len() <= 10 {
		// Write on one line.
		for i := 0; i < v.Len(); i++ {
			if i > 0 {
				s.printString(", ")
			}
			s.rprint(v.Index(i), t.Elem(), true)
		}
		s.printString("}")
	} else {
		s.printString("\n")
		for i := 0; i < v.Len(); i++ {
			s.printString("\t")
			s.rprint(v.Index(i), t.Elem(), true)
			s.printString(",\n")
		}
		s.printString("}\n")
	}
}

func (s *state) printMap(v reflect.Value, imputedType reflect.Type, elide bool) {
	if s.printIfNil(v, imputedType) {
		return
	}
	t := v.Type()
	var ts string
	if elide && t == imputedType {
		ts = ""
	} else {
		ts = s.sprintType(t)
	}
	keys := v.MapKeys()
	// Sort the keys if we can.
	if less := lessFunc(t.Key()); less != nil {
		sort.Slice(keys, func(i, j int) bool {
			return less(keys[i], keys[j])
		})
	}

	multiline := len(keys) >= 2
	s.printf("%s{", ts)
	if multiline {
		s.printString("\n")
	}
	for i, k := range keys {
		if multiline {
			s.printString("\t")
		}
		if i > 0 && !multiline {
			s.printString(", ")
		}
		s.rprint(k, t.Key(), true)
		s.printString(": ")
		s.rprint(v.MapIndex(k), t.Elem(), true)
		if multiline {
			s.printString(",\n")
		}
	}
	s.printString("}")
	if multiline {
		s.printString("\n")
	}
}

func (s *state) printStruct(rv reflect.Value, imputedType reflect.Type, elide bool) {
	t := rv.Type()
	var ts string
	if elide && t == imputedType {
		ts = ""
	} else {
		ts = s.sprintType(t)
	}

	var inds []int
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).IsExported() && !rv.Field(i).IsZero() {
			inds = append(inds, i)
		}
	}
	switch len(inds) {
	case 0:
		s.printf("%s{}", ts)
	case 1:
		i := inds[0]
		s.printf("%s{%s: %s}", ts, t.Field(i).Name, s.rsprint(rv.Field(i), t.Field(i).Type, false))
	default:
		s.printf("%s{\n", ts)
		for _, i := range inds {
			s.printf("\t%s: ", t.Field(i).Name)
			s.rprint(rv.Field(i), t.Field(i).Type, false)
			s.printf(",\n")
		}
		s.printf("}\n")
	}
}

func (s *state) sprintType(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Ptr:
		return "*" + s.sprintType(t.Elem())
	case reflect.Slice:
		return "[]" + s.sprintType(t.Elem())
	case reflect.Array:
		return fmt.Sprintf("[%d]", t.Len()) + s.sprintType(t.Elem())
	case reflect.Map:
		return "map[" + s.sprintType(t.Key()) + "]" + s.sprintType(t.Elem())
	case reflect.Interface:
		if t.NumMethod() == 0 {
			return "interface{}"
		}
	}
	if t.Name() == "" {
		s.err = fmt.Errorf("can't handle unnamed type %s", t)
		return "???"
	}
	pkgPath := t.PkgPath()
	if pkgPath == "" {
		return t.String()
	}
	if pkgPath == s.p.pkgPath {
		return t.Name()
	}
	if id, ok := s.p.imports[pkgPath]; ok {
		return id + "." + t.Name()
	}
	s.err = fmt.Errorf("unknown package %s; call Printer.RegisterImport(%[1]q, <identifier>)", pkgPath)
	return "???"
}

func (s *state) printIfNil(v reflect.Value, imputedType reflect.Type) bool {
	switch v.Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Interface:
		if v.IsNil() {
			if v.Type() == imputedType {
				s.printString("nil")
			} else {
				ts := s.sprintType(v.Type())
				if ts[0] == '*' {
					s.printf("(%s)(nil)", ts)
				} else {
					s.printf("%s(nil)", ts)
				}
			}
			return true
		}
	}
	return false
}

func (s *state) printString(str string) {
	if s.err == nil {
		_, s.err = io.WriteString(s.w, str)
	}
}

func (s *state) printf(format string, args ...interface{}) {
	if s.err == nil {
		_, s.err = fmt.Fprintf(s.w, format, args...)
	}
}

func isPrimitive(k reflect.Kind) bool {
	switch k {
	case reflect.Bool, reflect.String, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return true
	default:
		return false
	}
}

// default type for an untyped constant
func defaultType(v reflect.Value) reflect.Type {
	switch {
	case v.Kind() == reflect.Bool:
		return tBool
	case v.Kind() == reflect.String:
		return tString
	case isInt(v.Type()) || isUint(v.Type()):
		return tInt
	case isFloat(v.Type()):
		return tFloat64
	case isComplex(v.Type()):
		return tComplex128
	default:
		panic("not a possible value for a numeric untyped constant")
	}
}

func isInt(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	}
	return false
}

func isUint(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return true
	}
	return false
}

func isFloat(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

func isComplex(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Complex64, reflect.Complex128:
		return true
	}
	return false
}

// func isNumeric(t reflect.Type) bool {
// 	if t == nil {
// 		return false
// 	}
// 	k := t.Kind()
// 	if k == reflect.Bool || k == reflect.String {
// 		return false
// 	}
// 	return isPrimitive(k)
// }

func lessFunc(t reflect.Type) func(v1, v2 reflect.Value) bool {
	switch t.Kind() {
	case reflect.Bool:
		return func(v1, v2 reflect.Value) bool {
			return !v1.Bool() && v2.Bool()
		}
	case reflect.String:
		return func(v1, v2 reflect.Value) bool {
			return v1.String() < v2.String()
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return func(v1, v2 reflect.Value) bool {
			return v1.Int() < v2.Int()
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return func(v1, v2 reflect.Value) bool {
			return v1.Uint() < v2.Uint()
		}

	case reflect.Float32, reflect.Float64:
		return func(v1, v2 reflect.Value) bool {
			return v1.Float() < v2.Float()
		}

	default:
		return nil
	}
}
