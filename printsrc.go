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
	"strings"
	"time"
)

// A Printer prints Go values as source code.
type Printer struct {
	pkgPath   string
	imports   map[string]string // from package path to identifier
	printers  map[reflect.Type]printFunc
	lessFuncs map[reflect.Type]lessFunc
}

// NewPrinter constructs a Printer. The argument is the import path of the
// package where the printed code will reside.
//
// A custom printer for time.Time is registered by default. To override
// it or to add custom printers for other types, call RegisterPrinter.
func NewPrinter(packagePath string) *Printer {
	p := &Printer{
		pkgPath:   packagePath,
		imports:   map[string]string{},
		printers:  map[reflect.Type]printFunc{},
		lessFuncs: map[reflect.Type]lessFunc{},
	}
	p.RegisterPrinter(func(t time.Time) (string, error) {
		loc := t.Location()
		if loc != time.Local && loc != time.UTC {
			return "", fmt.Errorf("don't know how to represent location %q in source", loc)
		}
		ident := p.PackageIdentifier("time")
		return fmt.Sprintf("%s.Date(%d, %[1]s.%[3]s, %d, %d, %d, %d, %d, %[1]s.%[9]s)",
				ident,
				t.Year(), t.Month(), t.Day(),
				t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), loc),
			nil
	})
	return p
}

// RegisterImport tells the Printer to use the given identifier when
// printing types imported from packagePath.
func (p *Printer) RegisterImport(packagePath, ident string) {
	p.imports[packagePath] = ident
}

// PackageIdentifier returns the identifier that should prefix type names from
// the given import path. It returns the empty string if pkgPath is the same as
// the path given to NewPrinter. Otherwise, if an identifier has been provided
// with RegisterImport, it uses that. Finally, it returns the last component of
// the import path.
func (p *Printer) PackageIdentifier(pkgPath string) string {
	if pkgPath == p.pkgPath {
		return ""
	}
	if ident, ok := p.imports[pkgPath]; ok {
		return ident
	}
	// Assume the package identifier is the last component of the package path.
	// That is not always correct, which is why Printer.RegisterImport can override it.
	return path.Base(pkgPath)
}

// RegisterPrinter associates a type with a function that renders values of that
// type as Go source. An existing function for the type is replaced.
//
// The printFunc argument must be a function of one argument. Values with the
// type of that argument will be rendered with the function instead of in the
// usual way. The function's return type must be string or (string, error).
// RegisterPrinter panics if the function signature is invalid.
func (p *Printer) RegisterPrinter(printFunc interface{}) {
	argType, fun, err := processPrintFunc(printFunc)
	if err != nil {
		panic(err)
	}
	p.printers[argType] = fun
}

// type for wrapped custom print functions.
type printFunc func(reflect.Value) (string, error)

var tError = reflect.TypeOf([]error(nil)).Elem()

func processPrintFunc(pf interface{}) (argType reflect.Type, f printFunc, err error) {
	fv := reflect.ValueOf(pf)
	ft := fv.Type()
	if ft.Kind() != reflect.Func {
		return nil, nil, fmt.Errorf("argument to RegisterPrinter not a function: %v", pf)
	}
	if ft.NumIn() != 1 || ft.IsVariadic() {
		return nil, nil, fmt.Errorf("argument to RegisterPrinter must be a function of one non-variadic argument: %v", pf)
	}
	argType = ft.In(0)
	if !((ft.NumOut() == 1 && ft.Out(0) == tString) ||
		(ft.NumOut() == 2 && ft.Out(0) == tString && ft.Out(1) == tError)) {
		return nil, nil, fmt.Errorf("argument to RegisterPrinter must be a function returning string or (string, error): %v", pf)
	}
	f = func(v reflect.Value) (string, error) {
		outs := fv.Call([]reflect.Value{v})
		var err error
		if len(outs) == 2 {
			err, _ = outs[1].Interface().(error) // If it's not an error, it's nil.
		}
		return outs[0].String(), err
	}
	return argType, f, nil
}

// RegisterLess associates a function with a type that will be used to sort map
// keys of that type.
//
// When rendering a map value as Go source, printsrc will sort the keys if it
// can figure out how. By default it can sort any type whose underlying type is
// numeric, string or bool. Maps with other key types will have their keys
// printed in random order, complicating diffs. If a less function is registered
// for a key type, however, then map keys of that type will be sorted.
//
// The provided lessFunc must be a function of two arguments, both of the same
// type, and a single bool return value. It should report whether its first
// argument is less than its second. When confronted with a map whose key type
// is the function's argument type, printsrc will use the registered function to
// sort the keys.
//
// RegisterLess panics if the function signature is invalid.
//
// RegisterLess can be used to override the built-in less functions.
func (p *Printer) RegisterLess(lessFunc interface{}) {
	argType, fun, err := processLessFunc(lessFunc)
	if err != nil {
		panic(err)
	}
	p.lessFuncs[argType] = fun
}

type lessFunc func(v1, v2 reflect.Value) bool

func processLessFunc(lf interface{}) (argType reflect.Type, f lessFunc, err error) {
	fv := reflect.ValueOf(lf)
	ft := fv.Type()
	if ft.Kind() != reflect.Func {
		return nil, nil, fmt.Errorf("argument to RegisterLess not a function: %v", lf)
	}
	if ft.NumIn() != 2 || ft.IsVariadic() || ft.In(0) != ft.In(1) {
		return nil, nil, fmt.Errorf("argument to RegisterLess must be a function of two non-variadic arguments of the same type: %v", lf)
	}
	argType = ft.In(0)
	if ft.NumOut() != 1 || ft.Out(0) != tBool {
		return nil, nil, fmt.Errorf("argument to RegisterLess must be a function returning bool: %v", lf)
	}
	f = func(v1, v2 reflect.Value) bool {
		outs := fv.Call([]reflect.Value{v1, v2})
		return outs[0].Interface().(bool)
	}
	return argType, f, nil
}

func (p *Printer) getLessFunc(t reflect.Type) func(v1, v2 reflect.Value) bool {
	if f, ok := p.lessFuncs[t]; ok {
		return f
	}
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

// Sprint returns a string that is a valid Go expression for value.
// See the template example for how to use Sprint with text/template
// to generate code.
func (p *Printer) Sprint(value interface{}) (string, error) {
	var buf bytes.Buffer
	if err := p.Fprint(&buf, value); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Fprint prints a valid Go expression for value to w.
func (p *Printer) Fprint(w io.Writer, value interface{}) error {
	s := &state{
		w: w,
		p: p,
	}
	s.print(reflect.ValueOf(value), nil, false)
	return s.err
}

// Internal state for printing.
type state struct {
	p        *Printer
	w        io.Writer
	err      error
	depth    int // recursive calls to print
	tabDepth int // tabs from printSeq
}

// Fail after this many recursive calls to state.print.
const maxDepth = 100

// print is the main printing function. In addition to a value, it takes a type
// that constants will be automatically converted to (the "imputed type"). It
// also takes a boolean saying whether printing the imputed type can be elided.
func (s *state) print(v reflect.Value, imputedType reflect.Type, elide bool) {
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
		s.printString("nil")
		return
	}
	if cp := s.p.printers[v.Type()]; cp != nil {
		out, err := cp(v)
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
		s.print(v.Elem(), imputedType, elide)
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

func (s *state) sprint(v reflect.Value, imputedType reflect.Type, elide bool) string {
	s2 := *s
	var buf bytes.Buffer
	s2.w = &buf
	s2.print(v, imputedType, elide)
	return buf.String()
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
	if v.Kind() == reflect.Float64 && !strings.ContainsAny(vs, ".e") {
		vs += ".0"
	}
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
			s.sprintType(elem.Type()), s.sprint(elem, elem.Type(), false))
	} else if v.Type() == imputedType && elide {
		s.print(elem, imputedType.Elem(), elide)
	} else {
		s.printString("&")
		s.print(elem, nil, false)
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
	s.printSeq(!oneLineValue(v), v.Len(), func(i int) {
		s.print(v.Index(i), t.Elem(), true)
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
	if less := s.p.getLessFunc(t.Key()); less != nil {
		sort.Slice(keys, func(i, j int) bool {
			return less(keys[i], keys[j])
		})
	}
	s.printString(ts)
	s.printSeq(!oneLineValue(v), len(keys), func(i int) {
		s.print(keys[i], t.Key(), true)
		s.printString(": ")
		s.print(v.MapIndex(keys[i]), t.Elem(), true)
	})
}

func (s *state) printStruct(v reflect.Value, imputedType reflect.Type, elide bool) {
	t := v.Type()
	var (
		inds      []int
		multiline = false
	)
	for i := 0; i < t.NumField(); i++ {
		if (t.PkgPath() == s.p.pkgPath || isExported(t.Field(i))) && !v.Field(i).IsZero() {
			inds = append(inds, i)
			if !oneLineType(t.Field(i).Type) {
				multiline = true
			}
		}
	}
	if len(inds) == 0 && !v.IsZero() {
		s.err = fmt.Errorf("non-zero %s struct has no printable fields; call Printer.RegisterPrinter(%[1]s{}, ...)", t)
		return
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
		s.print(v.Field(ind), t.Field(ind).Type, false)
	})
}

func isExported(f reflect.StructField) bool {
	return f.PkgPath == ""
}

// printSeq prints a sequence of values (slice elements, array elements, or map key-value pairs).
// If multiline is true, each value is printed on its own line.
// Otherwise, all values are printed on a single line.
func (s *state) printSeq(multiline bool, n int, printElem func(int)) {
	printPrefix := func() {
		s.printString("\n")
		for i := 0; i < s.tabDepth; i++ {
			s.printString("\t")
		}
	}

	s.printString("{")
	if multiline {
		s.tabDepth++
		for i := 0; i < n; i++ {
			printPrefix()
			printElem(i)
			s.printString(",")
		}
		s.tabDepth--
		printPrefix()
		s.printString("}")
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

// sprintType returns a string denoting the given type.
func (s *state) sprintType(t reflect.Type) string {
	if t.Name() != "" {
		pkgPath := t.PkgPath()
		if pkgPath == "" {
			return t.String()
		}
		ident := s.p.PackageIdentifier(pkgPath)
		if ident == "" {
			return t.Name()
		}
		return ident + "." + t.Name()
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

func oneLineValue(v reflect.Value) bool {
	if oneLineType(v.Type()) {
		return true
	}
	if v.IsZero() {
		return true
	}
	switch v.Kind() {
	case reflect.String:
		return v.Len() <= 20
	case reflect.Slice, reflect.Array:
		return v.Len() == 0 ||
			(v.Len() == 1 && oneLineValue(v.Index(0))) ||
			(v.Len() <= 10 && oneLineType(v.Type().Elem()))
	case reflect.Map:
		if v.Len() == 0 {
			return true
		}
		k0 := v.MapKeys()[0]
		return (v.Len() == 1 && oneLineValue(k0) && oneLineValue(v.MapIndex(k0))) ||
			(v.Len() <= 5 && oneLineType(v.Type().Key()) && oneLineType(v.Type().Elem()))
	default:
		return false
	}
}

func oneLineType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.String:
		return false
	case reflect.Ptr:
		// Although pointers might seem small, we generate expressions for pointers that
		// are large (inline functions).
		return false
	case reflect.Struct:
		switch t.NumField() {
		case 0:
			return true
		case 1:
			return oneLineType(t.Field(0).Type)
		case 2:
			return oneLineType(t.Field(0).Type) && oneLineType(t.Field(1).Type)
		default:
			return false
		}
	default:
		return isPrimitive(t.Kind())
	}
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
