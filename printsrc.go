// Copyright 2021 by Jonathan Amsterdam. All rights reserved.

package printsrc

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"path"
	"reflect"
	"sort"
	"time"
)

const maxDepth = 30

type Printer struct {
	pkgPath        string
	customPrinters map[reflect.Type]PrintFunc
	imports        map[string]string // from package path to identifier
}

func NewPrinter(packagePath string) *Printer {
	p := &Printer{
		pkgPath:        packagePath,
		customPrinters: map[reflect.Type]PrintFunc{},
		imports:        map[string]string{},
	}
	p.RegisterCustomPrinter(time.Time{}, func(x interface{}) (string, error) {
		if err := p.CheckImport("time"); err != nil {
			return "", err
		}
		t := x.(time.Time)
		loc := t.Location()
		if loc != time.Local && loc != time.UTC {
			return "", fmt.Errorf("don't know how to represent location %q in source", loc)
		}
		return fmt.Sprintf("time.Date(%d, time.%s, %d, %d, %d, %d, %d, time.%s)",
				t.Year(), t.Month(), t.Day(),
				t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), loc),
			nil
	})
	return p
}

func (p *Printer) RegisterImport(packagePath string) {
	p.RegisterNamedImport(packagePath, path.Base(packagePath))
}

func (p *Printer) RegisterNamedImport(packagePath, ident string) {
	p.imports[packagePath] = ident
}

func (p *Printer) CheckImport(pkgPath string) error {
	if _, ok := p.imports[pkgPath]; !ok {
		return fmt.Errorf("unknown package %s; call Printer.RegisterImport(%[1]q, <identifier>)", pkgPath)
	}
	return nil
}

type PrintFunc func(interface{}) (string, error)

func (p *Printer) RegisterCustomPrinter(valueForType interface{}, f PrintFunc) {
	p.customPrinters[reflect.TypeOf(valueForType)] = f
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
		w:     w,
		p:     p,
		err:   nil,
		depth: 0,
	}
	s.rprint(reflect.ValueOf(value), nil, false)
	return s.err
}

type state struct {
	p     *Printer
	w     io.Writer
	err   error
	depth int
}

func (s *state) rsprint(v reflect.Value, imputedType reflect.Type, elide bool) string {
	s2 := *s
	var buf bytes.Buffer
	s2.w = &buf
	s2.rprint(v, imputedType, elide)
	return buf.String()
}

func (s *state) rprint(v reflect.Value, imputedType reflect.Type, elide bool) {
	if s.err != nil {
		return
	}
	if s.depth > maxDepth {
		s.err = errors.New("max recursion depth exceeded (probable circularity)")
		return
	}
	s.depth++
	defer func() { s.depth-- }()

	if !v.IsValid() {
		s.printString("nil") // TODO: may need a cast
		return
	}
	if cp := s.p.customPrinters[v.Type()]; cp != nil {
		out, err := cp(v.Interface())
		if err != nil {
			s.err = err
			return
		}
		s.printString(out)
		return
	}

	switch v.Kind() {
	case reflect.Bool, reflect.String, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		s.printPrimitiveLiteral(v, imputedType)
	case reflect.Float32, reflect.Float64:
		s.printFloat(v, imputedType)
	case reflect.Complex64, reflect.Complex128:
		s.printComplex(v, imputedType)
	case reflect.Ptr:
		s.printPtr(v, imputedType, elide)
	case reflect.Interface:
		s.rprint(v.Elem(), imputedType, elide)
	case reflect.Slice, reflect.Array:
		s.printSliceOrArray(v, imputedType, elide)
	case reflect.Map:
		s.printMap(v, imputedType, elide)
	case reflect.Struct:
		s.printStruct(v, imputedType, elide)
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		s.err = fmt.Errorf("cannot print values of type %s as source", v.Type())
	default:
		panic("bad kind")
	}
}

var (
	tBool       = reflect.TypeOf(false)
	tString     = reflect.TypeOf("")
	tInt        = reflect.TypeOf(int(0))
	tFloat64    = reflect.TypeOf(float64(0))
	tComplex128 = reflect.TypeOf(complex128(0))
)

func (s *state) printPrimitiveLiteral(v reflect.Value, imputedType reflect.Type) {
	// If a constant occurs in a context where it won't be converted
	// to the right type, we need an explicit conversion.
	// The value could have been in a location whose underlying type
	// causes an implicit conversion, or whose underlying type is interface.
	// Or the value might not have come from a location at all: it might be at top level.
	vs := fmt.Sprintf("%#v", v)
	if v.Type() != defaultType(v) && (imputedType == nil || imputedType.Kind() == reflect.Interface) {
		s.printf("%s(%s)", s.sprintType(v.Type()), vs)
	} else {
		s.printString(vs)
	}
}

func (s *state) printFloat(v reflect.Value, imputedType reflect.Type) {
	f := v.Float()
	fs := specialFloatString(f)
	if fs == "" {
		s.printPrimitiveLiteral(v, imputedType)
		return
	}
	if v.Type() == tFloat64 {
		s.printString(fs)
	} else {
		// We can't omit the conversion here, regardless of the imputed type, because
		// this is not a constant literal.
		s.printf("%s(%s)", s.sprintType(v.Type()), fs)
	}
}

func specialFloatString(f float64) string {
	switch {
	case math.IsNaN(f):
		return "math.NaN()"
	case math.IsInf(f, 1):
		return "math.Inf(1)"
	case math.IsInf(f, -1):
		return "math.Inf(-1)"
	default:
		return ""
	}
}

func (s *state) printComplex(v reflect.Value, imputedType reflect.Type) {
	c := v.Complex()
	rs := specialFloatString(real(c))
	is := specialFloatString(imag(c))
	if rs == "" && is == "" {
		s.printPrimitiveLiteral(v, imputedType)
	} else {
		vs := fmt.Sprintf("complex(%s, %s)", rs, is)
		// vs always represents a complex128. We do a conversion whenever the value
		// is of any other type.
		if v.Type() != tComplex128 {
			s.printf("%s(%s)", s.sprintType(v.Type()), vs)
		} else {
			s.printString(vs)
		}
	}
}

func (s *state) printPtr(v reflect.Value, imputedType reflect.Type, elide bool) {
	elem := v.Elem()
	if s.printIfNil(v, imputedType) {
		return
	}
	if isPrimitive(elem.Kind()) {
		s.printf("func() *%s { var x %[1]s = %s; return &x }()",
			s.sprintType(elem.Type()), s.rsprint(elem, elem.Type(), false))
	} else if v.Type() == imputedType && elide {
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

	s.printString(ts)
	s.printSeq(!isSmall(t.Elem()) || v.Len() > 10, v.Len(), func(i int) {
		s.rprint(v.Index(i), t.Elem(), true)
	})
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

	oneline := len(keys) < 2 || (len(keys) == 2 && isSmall(t.Key()) && isSmall(t.Elem()))
	s.printString(ts)
	s.printSeq(!oneline, len(keys), func(i int) {
		s.rprint(keys[i], t.Key(), true)
		s.printString(": ")
		s.rprint(v.MapIndex(keys[i]), t.Elem(), true)
	})
}

func (s *state) printStruct(v reflect.Value, imputedType reflect.Type, elide bool) {
	t := v.Type()
	var (
		inds      []int
		multiline = false
	)
	for i := 0; i < t.NumField(); i++ {
		if (t.PkgPath() == s.p.pkgPath || t.Field(i).IsExported()) && !v.Field(i).IsZero() {
			inds = append(inds, i)
			if !isSmall(t.Field(i).Type) {
				multiline = true
			}
		}
	}
	if len(inds) < 2 {
		multiline = false
	}
	if !(elide && t == imputedType) {
		s.printString(s.sprintType(t))
	}
	s.printSeq(multiline, len(inds), func(i int) {
		ind := inds[i]
		s.printf("%s: ", t.Field(ind).Name)
		s.rprint(v.Field(ind), t.Field(ind).Type, false)
	})
}

func (s *state) printSeq(multiline bool, n int, printElem func(int)) {
	s.printString("{")
	if multiline {
		for i := 0; i < n; i++ {
			s.printString("\n\t")
			printElem(i)
			s.printString(",")
		}
		s.printString("\n}\n")
	} else {
		for i := 0; i < n; i++ {
			if i > 0 {
				s.printString(", ")
			}
			printElem(i)
		}
		s.printString("}")
	}
}

func (s *state) sprintType(t reflect.Type) string {
	if t.Name() != "" {
		pkgPath := t.PkgPath()
		if pkgPath == "" {
			return t.String()
		}
		if pkgPath == s.p.pkgPath {
			return t.Name()
		}
		if err := s.p.CheckImport(pkgPath); err != nil {
			s.err = err
			return ""
		}
		return s.p.imports[pkgPath] + "." + t.Name()
	}
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
	s.err = fmt.Errorf("can't handle unnamed type %s", t)
	return ""
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

func isSmall(t reflect.Type) bool {
	return isPrimitive(t.Kind()) && t.Kind() != reflect.String
}

// default type for an untyped constant
func defaultType(v reflect.Value) reflect.Type {
	switch v.Kind() {
	case reflect.Bool:
		return tBool
	case reflect.String:
		return tString
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return tInt
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		// A uint, printed as a literal, will act like any other integer literal.
		return tInt
	case reflect.Float32, reflect.Float64:
		return tFloat64
	case reflect.Complex64, reflect.Complex128:
		return tComplex128
	default:
		panic("not a possible value for a numeric untyped constant")
	}
}

// func isInt(t reflect.Type) bool {
// 	switch t.Kind() {
// 	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
// 		return true
// 	}
// 	return false
// }

// func isUint(t reflect.Type) bool {
// 	switch t.Kind() {
// 	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
// 		return true
// 	}
// 	return false
// }

// func isFloat(t reflect.Type) bool {
// 	switch t.Kind() {
// 	case reflect.Float32, reflect.Float64:
// 		return true
// 	}
// 	return false
// }

// func isComplex(t reflect.Type) bool {
// 	switch t.Kind() {

// 		return true
// 	}
// 	return false
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
