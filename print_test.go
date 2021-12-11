package main

import (
	"math"
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
	B int
}

func TestPrint(t *testing.T) {
	i := int8(7)
	fn := Float(math.NaN())
	fn32 := float32(math.NaN())
	for _, test := range []struct {
		in   interface{}
		want string
	}{
		{nil, "nil"},
		{5, "5"},
		{-87, "-87"},
		{int32(3), "3"},
		{3.2, "3.2"},
		{1e-5, "1e-05"},
		{false, "false"},
		{true, "true"},
		{"a\tb", `"a\tb"`},
		{math.NaN(), "math.NaN()"},
		{math.Inf(3), "math.Inf(1)"},
		{math.Inf(-3), "math.Inf(-1)"},
		{complex(1, -1), "complex(1, -1)"},
		{&i, "func() *int8 { var x int8 = 7; return &x }()"},
		{fn, "Float(math.NaN())"},
		{&fn, "func() *Float { var x Float = Float(math.NaN()); return &x }()"},
		{[]int{1, 2}, "[]int{1, 2}"},
		{[]Float{2.3}, "[]Float{2.3}"},
		{[]float32{fn32}, "[]float32{float32(math.NaN())}"},
		{[]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}, "[]int{\n\t1,\n\t2,\n\t3,\n\t4,\n\t5,\n\t6,\n\t7,\n\t8,\n\t9,\n\t10,\n\t11,\n}\n"},
		{[1]bool{true}, "[1]bool{true}"},
		{[]string{"a", "b"}, "[]string{\n\t\"a\",\n\t\"b\",\n}\n"},
		{map[string]int{}, "map[string]int{}"},
		{map[string]int{"a": 1}, `map[string]int{"a": 1}`},
		//{map[string]int{"a": 1, "b": 2}, "map[string]int{\n\t\"a\": 1,\n\t\"b\": 2,\n}\n"},
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
		{map[Nested]Nested{{B: 1}: {B: 2}}, "map[Nested]Nested{{B: 1}: {B: 2}}"},
		{[]Float{Float(math.NaN())}, "[]Float{Float(math.NaN())}"},
		{[]Float{fn}, "[]Float{Float(math.NaN())}"},
		{
			[]interface{}{float32(1), fn32, map[string]int(nil)},
			"[]interface{}{\n\tfloat32(1),\n\tfloat32(math.NaN()),\n\tmap[string]int(nil),\n}\n",
		},
	} {
		got, err := Sprint(test.in)
		if err != nil {
			t.Fatal(err)
		}
		if got != test.want {
			t.Errorf("%#v: got\n%s\nwant\n%s", test.in, got, test.want)
		}
	}
}
