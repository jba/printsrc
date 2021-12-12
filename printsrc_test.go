// Copyright 2021 by Jonathan Amsterdam. All rights reserved.

package printsrc

import (
	"math"
	"net"
	"strings"
	"testing"
	"text/template"
	"time"
)

type T struct {
	Boo bool
	Map map[string]Float
}

type nesting struct {
	A int
	Nested
}

type Nested struct {
	B int16
}

type Unexp struct {
	E, u float64
}

type (
	Bool    bool
	String  string
	Int     int16
	Uint    uint8
	Float   float32
	Complex complex64
)

type Underlying struct {
	B Bool
	S String
	I Int
	U Uint
	F Float
	C Complex
}

type (
	Point struct {
		x, y float32
	}

	PPoint *Point
)

type node struct {
	v    int
	next *node
}

type MyMap map[string]int

func TestPrint(t *testing.T) {
	p := NewPrinter("github.com/jba/printsrc")
	p.RegisterImport("net")
	p.RegisterImport("time")
	p.RegisterNamedImport("text/template", "ttemp")
	i8 := int8(7)
	fn := Float(math.NaN())
	fn32 := float32(math.NaN())
	c128 := complex(1, -1)
	for _, test := range []struct {
		in   interface{}
		want string
	}{
		// simple primitive values
		{nil, "nil"},
		{"a\tb", `"a\tb"`},
		{true, "true"},
		{Bool(true), "Bool(true)"},
		{[]Bool{true}, "[]Bool{true}"},

		// integers
		{5, "5"},
		{-87, "-87"},
		{int32(3), "int32(3)"},

		{
			// Constant literals as field values of struct literals are implicitly converted.
			Underlying{B: true, S: "ok", I: 1, U: 2, F: 3, C: 4},
			`Underlying{B: true,S: "ok",I: 1,U: 0x2,F: 3,C: (4+0i),}`,
		},

		// floating-point
		{3.2, "3.2"},
		{1e-5, "1e-05"},
		{float32(1), "float32(1)"},
		{math.NaN(), "math.NaN()"},
		{math.Inf(3), "math.Inf(1)"},
		{math.Inf(-3), "math.Inf(-1)"},
		{[]float32{float32(math.NaN())}, "[]float32{float32(math.NaN())}"},

		// complex
		{complex(1, -1), "(1-1i)"},
		{c128, "(1-1i)"},
		{complex(float32(1), float32(-1)), "complex64((1-1i))"},
		{[]complex64{complex(1, -2)}, "[]complex64{(1-2i)}"},
		{[]Complex{complex(1, -2)}, "[]Complex{(1-2i)}"},
		{complex(math.NaN(), math.Inf(1)), "complex(math.NaN(), math.Inf(1))"},
		{
			[]complex64{complex64(complex(math.NaN(), math.Inf(1)))},
			"[]complex64{complex64(complex(math.NaN(), math.Inf(1)))}",
		},
		{
			[]Complex{Complex(complex(math.NaN(), math.Inf(1)))},
			"[]Complex{Complex(complex(math.NaN(), math.Inf(1)))}",
		},

		// pointers
		{(*int)(nil), "(*int)(nil)"},
		{&i8, "func() *int8 { var x int8 = 7; return &x }()"},
		{fn, "Float(math.NaN())"},
		{&fn, "func() *Float { var x Float = Float(math.NaN()); return &x }()"},
		{[]*int8{&i8}, "[]*int8{func() *int8 { var x int8 = 7; return &x }(),}"},
		{
			[]*int8{func() *int8 { var x int8 = 7; return &x }()},
			"[]*int8{func() *int8 { var x int8 = 7; return &x }(),}",
		},
		{[]*[]int{{1}}, "[]*[]int{{1},}"},

		// slices and arrays
		{[]int(nil), "[]int(nil)"},
		{[]int{}, "[]int{}"},
		{[]int{1}, "[]int{1}"},
		{[]int{1, 2}, "[]int{1, 2}"},
		{[]int{1, 2, 3}, "[]int{1, 2, 3}"},
		{[]int16{1, int16(i8)}, "[]int16{1, 7}"},
		{[]Float{2.3}, "[]Float{2.3}"},
		{[]float32{fn32}, "[]float32{float32(math.NaN())}"},
		{[1]bool{true}, "[1]bool{true}"},
		{[]string{"a", "b"}, `[]string{"a","b",}`},

		// maps
		{map[string]int(nil), "map[string]int(nil)"},
		{map[string]int{}, "map[string]int{}"},
		{map[string]int{"a": 1}, `map[string]int{"a": 1}`},
		{map[string]int{"a": 1, "b": 2}, `map[string]int{"a": 1,"b": 2,}`},
		{
			T{true, map[string]Float{"x": 0.5}},
			`T{Boo: true,Map: map[string]Float{"x": 0.5},}`,
		},
		{
			T{false, map[string]Float{"x": 0.5}},
			`T{Map: map[string]Float{"x": 0.5}}`,
		},
		{MyMap{"a": 1}, `MyMap{"a": 1}`},
		{map[int][]int{}, "map[int][]int{}"},
		{map[int][]int{1: {2}}, "map[int][]int{1: {2}}"},
		{map[int]int{1: 2, 3: 4}, "map[int]int{1: 2, 3: 4}"},
		{map[int][]int{1: {2}, 3: {4, 5, 6}}, "map[int][]int{1: {2},3: {4, 5, 6},}"},
		{map[bool]int{true: 1, false: 2}, "map[bool]int{false: 2, true: 1}"},
		{map[uint]bool{2: true, 1: false}, "map[uint]bool{0x1: false, 0x2: true}"},
		{map[float32]int{1.0: 1, -1.0: -1}, "map[float32]int{-1: -1, 1: 1}"},
		{[]map[int]bool{{1: true}}, "[]map[int]bool{{1: true},}"},

		// structs
		{(*Nested)(nil), "(*Nested)(nil)"},
		{&Nested{B: 3}, "&Nested{B: 3}"},
		{Point{1, 2}, "Point{x: 1, y: 2}"},
		{nesting{A: 1, Nested: Nested{B: 2}}, "nesting{A: 1,Nested: Nested{B: 2},}"},
		{[]Nested{{B: 1}}, "[]Nested{{B: 1},}"},
		{[]*Nested{{B: 1}, nil}, "[]*Nested{{B: 1},nil,}"},
		{Unexp{1, 2}, "Unexp{E: 1, u: 2}"},

		{map[Nested]Nested{{B: 1}: {B: 2}}, "map[Nested]Nested{{B: 1}: {B: 2}}"},
		{[]Float{Float(math.NaN())}, "[]Float{Float(math.NaN())}"},
		{[]Float{fn}, "[]Float{Float(math.NaN())}"},
		{
			[]interface{}{float32(1), fn32, map[string]int(nil)},
			"[]interface{}{float32(1),float32(math.NaN()),map[string]int(nil),}",
		},
		{
			&node{1, &node{2, &node{v: 3}}},
			"&node{v: 1,next: &node{v: 2,next: &node{v: 3},},}",
		},

		// imports
		// (net.Flags is a uint)
		{net.Flags(17), "net.Flags(0x11)"},
		// named import
		{template.Template{}, "ttemp.Template{}"},

		// elision examples from the Go spec
		{[...]Point{{1.5, -3.5}, {0, 0}}, "[2]Point{{x: 1.5, y: -3.5},{},}"},
		{[][]int{{1, 2, 3}, {4, 5}}, "[][]int{{1, 2, 3},{4, 5},}"},
		{[][]Point{{{0, 1}, {1, 2}}}, "[][]Point{{{y: 1},{x: 1, y: 2},},}"},
		{map[string]Point{"orig": {0, 0}}, `map[string]Point{"orig": {}}`},
		{map[Point]string{{0, 0}: "orig"}, `map[Point]string{{}: "orig"}`},
		{[2]*Point{{1.5, -3.5}, {}}, "[2]*Point{{x: 1.5, y: -3.5},{},}"},
		{[2]PPoint{{1.5, -3.5}, {}}, "[2]PPoint{{x: 1.5, y: -3.5},{},}"},

		// time.Time
		{
			time.Date(2008, 4, 23, 9, 56, 23, 29, time.Local),
			"time.Date(2008, time.April, 23, 9, 56, 23, 29, time.Local)",
		},
		{
			time.Date(2008, 4, 23, 9, 56, 23, 29, time.UTC),
			"time.Date(2008, time.April, 23, 9, 56, 23, 29, time.UTC)",
		},
	} {
		got, err := p.Sprint(test.in)
		if err != nil {
			t.Fatal(err)
		}
		got = strings.NewReplacer("\n", "", "\t", "").Replace(got)
		if got != test.want {
			t.Errorf("%#v (%[1]T):\ngot\n\t%s\nwant\n\t%s", test.in, got, test.want)
		}
	}
}

func TestPrintVerticalWhitespace(t *testing.T) {
	p := NewPrinter("github.com/jba/printsrc")
	for _, test := range []struct {
		in   interface{}
		want string
	}{
		{[]int{1, 2}, "[]int{1, 2}"},
		{[]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}, "[]int{\n\t1,\n\t2,\n\t3,\n\t4,\n\t5,\n\t6,\n\t7,\n\t8,\n\t9,\n\t10,\n\t11,\n}"},
		{[]string{"a", "b"}, "[]string{\n\t\"a\",\n\t\"b\",\n}"},
		{map[string]int{}, "map[string]int{}"},
		{map[string]int{"a": 1}, `map[string]int{"a": 1}`},
		{map[string]int{"a": 1, "b": 2}, "map[string]int{\n\t\"a\": 1,\n\t\"b\": 2,\n}"},
		{
			T{true, map[string]Float{"x": 0.5}},
			"T{\n\tBoo: true,\n\tMap: map[string]Float{\"x\": 0.5},\n}",
		},
		{
			T{false, map[string]Float{"x": 0.5}},
			"T{Map: map[string]Float{\"x\": 0.5}}",
		},
		{&Nested{B: 3}, "&Nested{B: 3}"},
		{nesting{A: 1, Nested: Nested{B: 2}}, "nesting{\n\tA: 1,\n\tNested: Nested{B: 2},\n}"},
		{[]Nested{{B: 1}}, "[]Nested{\n\t{B: 1},\n}"},
		{[]*Nested{{B: 1}}, "[]*Nested{\n\t{B: 1},\n}"},
		{map[int][]int{}, "map[int][]int{}"},
		{map[int][]int{1: {2}}, "map[int][]int{1: {2}}"},
		{map[int][]int{1: {2}, 3: {4, 5, 6}}, "map[int][]int{\n\t1: {2},\n\t3: {4, 5, 6},\n}"},
	} {
		got, err := p.Sprint(test.in)
		if err != nil {
			t.Fatal(err)
		}
		if got != test.want {
			t.Errorf("%#v (%[1]T):\ngot\n\t%s\nwant\n\t%s", test.in, got, test.want)
		}
	}
}

func TestPrintErrors(t *testing.T) {
	p := NewPrinter("github.com/jba/printsrc")
	p.RegisterImport("time")
	n := &node{v: 1}
	n.next = n

	for _, test := range []struct {
		in   interface{}
		want string
	}{
		{net.Flags(3), "unknown package"},
		{struct{ X int }{3}, "unnamed type"},
		{func() {}, "cannot print"},
		{make(chan int), "cannot print"},
		{n, "depth exceeded"},
		{time.Date(2008, 4, 23, 9, 56, 23, 29, time.FixedZone("foo", 17)), "location"},
	} {
		got, err := p.Sprint(test.in)
		if err == nil {
			t.Log("XXX", got)
			t.Errorf("%#v: got nil, want err", test.in)
		} else if !strings.Contains(err.Error(), test.want) {
			t.Errorf("%#v: got %q, looking for %q", test.in, err, test.want)
		}
	}
}
