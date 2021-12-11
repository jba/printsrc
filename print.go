// Print a Go value as Go source.

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

type state struct {
	w              io.Writer
	homePackage    string
	err            error
	customPrinters map[reflect.Type]PrintFunc
	importsNeeded  map[string]bool
}

func (s *state) addImport(pkg string) {
	if s.importsNeeded == nil {
		s.importsNeeded = map[string]bool{}
	}
	s.importsNeeded[pkg] = true
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
	s := &state{w: w}
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

func (s *state) rprint(rv reflect.Value) error {
	if !rv.IsValid() {
		return s.printString("nil") // TODO: may need a cast
	}
	if cp := s.customPrinters[rv.Type()]; cp != nil {
		out, err := cp(rv.Interface())
		if err != nil {
			return err
		}
		return s.printString(out)
	}

	switch rv.Kind() {
	case reflect.Bool, reflect.String, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return s.printf("%#v", rv.Interface())
	case reflect.Float32, reflect.Float64:
		return s.printFloat(rv)
	case reflect.Complex64, reflect.Complex128:
		//return s.printComplex(rv.Complex())
	case reflect.Ptr:
		// TODO: cycle detection
		return s.printPtr(rv.Elem())
	case reflect.Slice:
		//return s.printSlice(rv)
	case reflect.Array:
	case reflect.Interface:
	case reflect.Map:
	case reflect.Struct:
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return fmt.Errorf("cannot print %#v as source", rv.Interface())
	default:
		panic("bad kind")
	}
	return nil
}

func (s *state) rsprint(rv reflect.Value) (string, error) {
	s2 := *s
	var buf bytes.Buffer
	s2.w = &buf
	if err := s.rprint(rv); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (s *state) printFloat(rf reflect.Value) error {
	f := rf.Float()
	switch {
	case math.IsNaN(f):
		return s.printConversion(rf.Type(), reflect.TypeOf(float64(0)), "math.NaN()")
	case math.IsInf(f, 1):
		return s.printConversion(rf.Type(), reflect.TypeOf(float64(0)), "math.Inf(1)")
	case math.IsInf(f, -1):
		return s.printConversion(rf.Type(), reflect.TypeOf(float64(0)), "math.Inf(-1)")
	default:
		// No conversion needed for constant.
		return s.printf("%#v", f)
	}
}

func (s *state) printPtr(elem reflect.Value) error {
	switch elem.Kind() {
	case reflect.Bool, reflect.String, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:

		selem, err := s.rsprint(elem)
		if err != nil {
			return err
		}
		return s.printf("func() *%s { x := %[1]s(%s); return &x }()", elem.Type(), selem)
	default:
		if err := s.printString("&"); err != nil {
			return err
		}
		return s.rprint(elem)
	}
}

func (s *state) printString(str string) error {
	_, err := io.WriteString(s.w, str)
	return err
}

func (s *state) printf(format string, args ...interface{}) error {
	_, err := fmt.Fprintf(s.w, format, args...)
	return err
}

func err2(err1, err2 error) error {
	if err1 != nil {
		return err1
	}
	return err2
}

func (s *state) printConversion(t, b reflect.Type, expr string) error {
	if t == b || s.canElide(t) {
		return s.printString(expr)
	}
	return err2(s.printType(t), s.printf("(%s)", expr))
}

func (s *state) printType(t reflect.Type) error {
	if t.PkgPath() == s.homePackage {
		return s.printString(t.Name())
	}
	return s.printString(t.String())
}

func (s *state) canElide(t reflect.Type) bool {
	return false
}
