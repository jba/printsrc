package main

import (
	"math"
	"net"
	"strings"
	"testing"
)

type T struct {
	Boo bool
	Map map[string]Float
	u   int
}

type nesting struct {
	A int
	Nested
}

type Nested struct {
	B int16
}

type X struct {
	F float32
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

func TestPrint(t *testing.T) {
	p := NewPrinter("printing")
	p.RegisterImport("net", "net")
	i := int8(7)
	fn := Float(math.NaN())
	fn32 := float32(math.NaN())
	for _, test := range []struct {
		in   interface{}
		want string
	}{
		{nil, "nil"},

		// integers
		{5, "5"},
		{-87, "-87"},
		{int32(3), "int32(3)"},
		{3.2, "3.2"},
		{1e-5, "1e-05"},
		{true, "true"},
		{Bool(true), "Bool(true)"},
		{[]Bool{true}, "[]Bool{true}"},
		{
			// Constant literals as field values of struct literals are implicitly converted.
			Underlying{B: true, S: "ok", I: 1, U: 2, F: 3, C: 4},
			`Underlying{B: true,S: "ok",I: 1,U: 0x2,F: 3,C: complex(4, 0),}`,
		},
		{"a\tb", `"a\tb"`},
		{math.NaN(), "math.NaN()"},
		{math.Inf(3), "math.Inf(1)"},
		{math.Inf(-3), "math.Inf(-1)"},
		{complex(1, -1), "complex(1, -1)"},

		// pointers
		{(*int)(nil), "(*int)(nil)"},
		{&i, "func() *int8 { var x int8 = 7; return &x }()"},
		{fn, "Float(math.NaN())"},
		{&fn, "func() *Float { var x Float = Float(math.NaN()); return &x }()"},

		// slices and arrays
		{[]int(nil), "[]int(nil)"},
		{[]int{}, "[]int{}"},
		{[]int{1}, "[]int{1}"},
		{[]int{1, 2}, "[]int{1, 2}"},
		{[]int{1, 2, 3}, "[]int{1, 2, 3}"},
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
			T{true, map[string]Float{"x": 0.5}, 1},
			`T{Boo: true,Map: map[string]Float{"x": 0.5},}`,
		},
		{
			T{false, map[string]Float{"x": 0.5}, 1},
			`T{Map: map[string]Float{"x": 0.5}}`,
		},

		// structs
		{(*Nested)(nil), "(*Nested)(nil)"},
		{&Nested{B: 3}, "&Nested{B: 3}"},
		{nesting{A: 1, Nested: Nested{B: 2}}, "nesting{A: 1,Nested: Nested{B: 2},}"},
		{[]Nested{{B: 1}}, "[]Nested{{B: 1},}"},
		{[]*Nested{{B: 1}, nil}, "[]*Nested{{B: 1},nil,}"},
		{map[int][]int{}, "map[int][]int{}"},
		{map[int][]int{1: {2}}, "map[int][]int{1: {2}}"},
		{map[int][]int{1: {2}, 3: {4, 5, 6}}, "map[int][]int{1: {2},3: {4, 5, 6},}"},
		{map[Nested]Nested{{B: 1}: {B: 2}}, "map[Nested]Nested{{B: 1}: {B: 2}}"},
		{[]Float{Float(math.NaN())}, "[]Float{Float(math.NaN())}"},
		{[]Float{fn}, "[]Float{Float(math.NaN())}"},
		{
			[]interface{}{float32(1), fn32, map[string]int(nil)},
			"[]interface{}{float32(1),float32(math.NaN()),map[string]int(nil),}",
		},
		// net.Flags is a uint
		{net.Flags(17), "net.Flags(0x11)"},
		{float32(1), "float32(1)"},
		{X{F: 1}, "X{F: 1}"},
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
	p := NewPrinter("printing")
	for _, test := range []struct {
		in   interface{}
		want string
	}{
		{[]int{1, 2}, "[]int{1, 2}"},
		{[]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}, "[]int{\n\t1,\n\t2,\n\t3,\n\t4,\n\t5,\n\t6,\n\t7,\n\t8,\n\t9,\n\t10,\n\t11,\n}\n"},
		{[]string{"a", "b"}, "[]string{\n\t\"a\",\n\t\"b\",\n}\n"},
		{map[string]int{}, "map[string]int{}"},
		{map[string]int{"a": 1}, `map[string]int{"a": 1}`},
		{map[string]int{"a": 1, "b": 2}, "map[string]int{\n\t\"a\": 1,\n\t\"b\": 2,\n}\n"},
		{
			T{true, map[string]Float{"x": 0.5}, 1},
			"T{\n\tBoo: true,\n\tMap: map[string]Float{\"x\": 0.5},\n}\n",
		},
		{
			T{false, map[string]Float{"x": 0.5}, 1},
			"T{Map: map[string]Float{\"x\": 0.5}}",
		},
		{&Nested{B: 3}, "&Nested{B: 3}"},
		{nesting{A: 1, Nested: Nested{B: 2}}, "nesting{\n\tA: 1,\n\tNested: Nested{B: 2},\n}\n"},
		{[]Nested{{B: 1}}, "[]Nested{\n\t{B: 1},\n}\n"},
		{[]*Nested{{B: 1}}, "[]*Nested{\n\t{B: 1},\n}\n"},
		{map[int][]int{}, "map[int][]int{}"},
		{map[int][]int{1: {2}}, "map[int][]int{1: {2}}"},
		{map[int][]int{1: {2}, 3: {4, 5, 6}}, "map[int][]int{\n\t1: {2},\n\t3: {4, 5, 6},\n}\n"},
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
	p := NewPrinter("printing")
	_, err := p.Sprint(net.Flags(3))
	if err == nil {
		t.Error("got nil, want err")
	}
}

// From the Go spec:
// [...]Point{{1.5, -3.5}, {0, 0}}     // same as [...]Point{Point{1.5, -3.5}, Point{0, 0}}
// [][]int{{1, 2, 3}, {4, 5}}          // same as [][]int{[]int{1, 2, 3}, []int{4, 5}}
// [][]Point{{{0, 1}, {1, 2}}}         // same as [][]Point{[]Point{Point{0, 1}, Point{1, 2}}}
// map[string]Point{"orig": {0, 0}}    // same as map[string]Point{"orig": Point{0, 0}}
// map[Point]string{{0, 0}: "orig"}    // same as map[Point]string{Point{0, 0}: "orig"}

// type PPoint *Point
// [2]*Point{{1.5, -3.5}, {}}          // same as [2]*Point{&Point{1.5, -3.5}, &Point{}}
// [2]PPoint{{1.5, -3.5}, {}}          // same as [2]PPoint{PPoint(&Point{1.5, -3.5}), PPoint(&Point{})}
