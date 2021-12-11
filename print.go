// Print a Go value as Go source.

// TODO:
// - elision
// - imports
// - anonymous types
// - ptr cycle detection
// - omit zero-valued struct fields

package main

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
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
	s.rprint(reflect.ValueOf(value))
}

func (s *state) rprint(rv reflect.Value) {
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
		s.printFloat(rv)
	case reflect.Complex64, reflect.Complex128:
		s.printComplex(rv)
	case reflect.Ptr:
		s.printPtr(rv.Elem())
	case reflect.Interface:
		s.rprint(rv.Elem())
	case reflect.Slice, reflect.Array:
		s.printSliceOrArray(rv)
	case reflect.Map:
		s.printMap(rv)
	case reflect.Struct:
		s.printStruct(rv)
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		s.err = fmt.Errorf("cannot print %#v as source", rv.Interface())
	default:
		panic("bad kind")
	}
}

func (s *state) rsprint(rv reflect.Value) string {
	s2 := *s
	var buf bytes.Buffer
	s2.w = &buf
	s2.rprint(rv)
	return buf.String()
}

func (s *state) printFloat(rf reflect.Value) {
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
		// No conversion needed for constant.
		s.printf("%#v", f)
		return
	}
	s.printConversion(rf.Type(), tFloat64, str)
}

func (s *state) printComplex(rc reflect.Value) {
	c := rc.Complex()
	s.printf("complex(%s, %s)", s.rsprint(reflect.ValueOf(real(c))), s.rsprint(reflect.ValueOf(imag(c))))
}

func (s *state) printPtr(elem reflect.Value) {
	if isPrimitive(elem.Kind()) {
		s.printf("func() *%s { var x %[1]s = %s; return &x }()", s.sprintType(elem.Type()), s.rsprint(elem))
	} else {
		s.printString("&")
		s.rprint(elem)
	}
}

func (s *state) printSliceOrArray(rv reflect.Value) {
	k := rv.Type().Elem().Kind()
	s.printf("%s{", s.sprintType(rv.Type()))
	if k != reflect.String && isPrimitive(k) && rv.Len() <= 10 {
		// Write on one line.
		for i := 0; i < rv.Len(); i++ {
			if i > 0 {
				s.printString(", ")
			}
			s.rprint(rv.Index(i))
		}
		s.printString("}")
	} else {
		s.printString("\n")
		for i := 0; i < rv.Len(); i++ {
			s.printString("\t")
			s.rprint(rv.Index(i))
			s.printString(",\n")
		}
		s.printString("}\n")
	}
}

func (s *state) printMap(rv reflect.Value) {
	switch rv.Len() {
	case 0:
		s.printf("%s{}", s.sprintType(rv.Type()))
	case 1:
		s.printf("%s{", s.sprintType(rv.Type()))
		k := rv.MapKeys()[0]
		s.rprint(k)
		s.printString(": ")
		s.rprint(rv.MapIndex(k))
		s.printString("}")
	default:
		s.printf("%s{\n", s.sprintType(rv.Type()))
		iter := rv.MapRange()
		for iter.Next() {
			// BUG: multiple NaNs will result in duplicate key, I think.
			// TODO: random key order makes tests flaky.
			s.printString("\t")
			s.rprint(iter.Key())
			s.printString(": ")
			s.rprint(iter.Value())
			s.printString(",\n")
		}
		s.printString("}\n")
	}
}

func (s *state) printStruct(rv reflect.Value) {
	t := rv.Type()
	numExported := 0
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).IsExported() {
			numExported++
		}
	}
	if numExported < 2 {
		s.printf("%s{", s.sprintType(t))
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.IsExported() {
				s.printf("%s: ", f.Name)
				s.rprint(rv.Field(i))
			}
		}
		s.printString("}")
	} else {
		s.printf("%s{\n", s.sprintType(t))
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.IsExported() {
				s.printf("\t%s: ", f.Name)
				s.rprint(rv.Field(i))
				s.printf(",\n")
			}
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
	default:
		if t.Name() == "" {
			s.err = fmt.Errorf("can't handle unnamed type %s", t)
			return "???"
		}
		if t.PkgPath() == s.pkg {
			return t.Name()
		}
		return t.String()
	}
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
