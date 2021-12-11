// Print a Go value as Go source.

// TODO:
// - testing complex, with and w/o elision
// - imports
// - ptr cycle detection
// - NaN map keys?
// - anonymous struct types?
package main

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
	"sort"
)

type Option optFunc

type optFunc func(*state) error

type PrintFunc func(interface{}) (string, error)

type Import struct {
	Identifier string
	Path       string
}

type state struct {
	w              io.Writer
	pkg            string
	err            error
	customPrinters map[reflect.Type]PrintFunc
	importsNeeded  map[string]Import
}

func (s *state) addImport(importPath string) {
	if s.importsNeeded == nil {
		s.importsNeeded = map[string]Import{}
	}
	// s.importsNeeded[importPath] = Import{id, pkg}b
}

func Use(tv interface{}, f PrintFunc) Option {
	return func(s *state) error {
		if s.customPrinters == nil {
			s.customPrinters = map[reflect.Type]PrintFunc{}
		}
		s.customPrinters[reflect.TypeOf(tv)] = f
		return nil
	}
}

func Sprint(value interface{}, opts ...Option) (string, error) {
	var buf bytes.Buffer
	if err := Fprint(&buf, value, opts...); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func Print(value interface{}, opts ...Option) error {
	return Fprint(os.Stdout, value, opts...)
}

func Fprint(w io.Writer, value interface{}, opts ...Option) error {
	s := &state{
		w:   w,
		pkg: "printing", // TODO: generalize
	}
	for _, opt := range opts {
		if err := opt(s); err != nil {
			return err
		}
	}
	s.print(value)
	return s.err
}

func (s *state) print(value interface{}) {
	s.rprint(reflect.ValueOf(value), nil)
}

func (s *state) rprint(rv reflect.Value, elide reflect.Type) {
	if !rv.IsValid() {
		s.printString("nil") // TODO: may need a cast
		return
	}
	if cp := s.customPrinters[rv.Type()]; cp != nil {
		out, err := cp(rv.Interface())
		if err != nil {
			s.err = err
			return
		}
		s.printString(out)
		return
	}

	switch rv.Kind() {
	case reflect.Bool, reflect.String, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		s.printf("%#v", rv.Interface())
	case reflect.Float32, reflect.Float64:
		s.printFloat(rv, elide)
	case reflect.Complex64, reflect.Complex128:
		s.printComplex(rv)
	case reflect.Ptr:
		s.printPtr(rv, elide)
	case reflect.Interface:
		s.rprint(rv.Elem(), elide)
	case reflect.Slice, reflect.Array:
		s.printSliceOrArray(rv, elide)
	case reflect.Map:
		s.printMap(rv, elide)
	case reflect.Struct:
		s.printStruct(rv, elide)
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		s.err = fmt.Errorf("cannot print %#v as source", rv.Interface())
	default:
		panic("bad kind")
	}
}

func (s *state) rsprint(rv reflect.Value, elide reflect.Type) string {
	s2 := *s
	var buf bytes.Buffer
	s2.w = &buf
	s2.rprint(rv, elide)
	return buf.String()
}

func (s *state) printFloat(rf reflect.Value, elide reflect.Type) {
	f := rf.Float()
	tFloat64 := reflect.TypeOf(float64(0))
	var str string
	switch {
	case math.IsNaN(f):
		str = "math.NaN()"
	case math.IsInf(f, 1):
		str = "math.Inf(1)"
	case math.IsInf(f, -1):
		str = "math.Inf(-1)"
	default:
		// If the constant occurs in a context where it won't be converted
		// to the right float type, we need a cast.
		// TODO: adds a cast for a struct field, where it may not be needed.
		if rf.Type() != tFloat64 && (elide == nil || (elide.Kind() != reflect.Float64 && elide.Kind() != reflect.Float32)) {
			s.printf("%s(%#v)", s.sprintType(rf.Type()), f)
		} else {
			s.printf("%#v", f)
		}
		return
	}
	// We can't elide the type here.
	s.printConversion(rf.Type(), tFloat64, str)
}

func (s *state) printComplex(rc reflect.Value) {
	c := rc.Complex()
	s.printf("complex(%s, %s)",
		s.rsprint(reflect.ValueOf(real(c)), nil),
		s.rsprint(reflect.ValueOf(imag(c)), nil))
}

func (s *state) printPtr(rv reflect.Value, elide reflect.Type) {
	elem := rv.Elem()
	if isPrimitive(elem.Kind()) {
		s.printf("func() *%s { var x %[1]s = %s; return &x }()", s.sprintType(elem.Type()), s.rsprint(elem, nil))
	} else if rv.Type() == elide {
		s.rprint(elem, elide.Elem())
	} else {
		s.printString("&")
		s.rprint(elem, nil)
	}
}

func (s *state) printSliceOrArray(rv reflect.Value, elide reflect.Type) {
	k := rv.Type().Elem().Kind()
	if rv.Type() == elide {
		s.printString("{")
	} else {
		s.printf("%s{", s.sprintType(rv.Type()))
	}
	if k != reflect.String && isPrimitive(k) && rv.Len() <= 10 {
		// Write on one line.
		for i := 0; i < rv.Len(); i++ {
			if i > 0 {
				s.printString(", ")
			}
			s.rprint(rv.Index(i), rv.Type().Elem())
		}
		s.printString("}")
	} else {
		s.printString("\n")
		for i := 0; i < rv.Len(); i++ {
			s.printString("\t")
			s.rprint(rv.Index(i), rv.Type().Elem())
			s.printString(",\n")
		}
		s.printString("}\n")
	}
}

func (s *state) printMap(rv reflect.Value, elide reflect.Type) {
	typ := rv.Type()
	ts := ""
	if typ != elide {
		ts = s.sprintType(typ)
	}
	switch rv.Len() {
	case 0:
		if rv.IsNil() {
			s.printf("%s(nil)", ts)
		} else {
			s.printf("%s{}", ts)
		}
	case 1:
		s.printf("%s{", ts)
		k := rv.MapKeys()[0]
		s.rprint(k, typ.Key())
		s.printString(": ")
		s.rprint(rv.MapIndex(k), typ.Elem())
		s.printString("}")
	default:
		keys := rv.MapKeys()
		if less := lessFunc(typ.Key()); less != nil {
			sort.Slice(keys, func(i, j int) bool {
				return less(keys[i], keys[j])
			})
		}
		s.printf("%s{\n", ts)
		for _, k := range keys {
			s.printString("\t")
			s.rprint(k, typ.Key())
			s.printString(": ")
			s.rprint(rv.MapIndex(k), typ.Elem())
			s.printString(",\n")
		}
		s.printString("}\n")
	}
}

func (s *state) printStruct(rv reflect.Value, elide reflect.Type) {
	typ := rv.Type()

	type field struct {
		name  string
		value reflect.Value
	}

	var fields []field
	for i := 0; i < typ.NumField(); i++ {
		if typ.Field(i).IsExported() {
			v := rv.Field(i)
			if !v.IsZero() {
				fields = append(fields, field{typ.Field(i).Name, v})
			}
		}
	}
	stype := ""
	if typ != elide {
		stype = s.sprintType(typ)
	}
	switch len(fields) {
	case 0:
		s.printf("%s{}", stype)
	case 1:
		s.printf("%s{%s: %s}", stype, fields[0].name, s.rsprint(fields[0].value, nil))
	default:
		s.printf("%s{\n", stype)
		for _, f := range fields {
			s.printf("\t%s: ", f.name)
			s.rprint(f.value, nil)
			s.printf(",\n")
		}
		s.printf("}\n")
	}
}

func (s *state) canElide(t reflect.Type) bool {
	return false
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
	if t.PkgPath() == s.pkg {
		return t.Name()
	}
	return t.String()
}

func (s *state) printConversion(t, b reflect.Type, expr string) {
	if t == b || s.canElide(t) {
		s.printString(expr)
	} else {
		s.printf("%s(%s)", s.sprintType(t), expr)
	}
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
