package main

import (
	"math"
	"testing"
)

func TestPrint(t *testing.T) {
	i := int8(7)
	f := Float(math.NaN())
	for _, test := range []struct {
		in   interface{}
		want string
	}{
		{nil, "nil"}, // TODO: conversion may be needed
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
		{&i, "func() *int8 { x := int8(7); return &x }()"},
		{f, "Float(math.NaN())"},
		{&f, "func() *Float { x := Float(math.NaN()); return &x }()"},
	} {
		got, err := Sprint(test.in)
		if err != nil {
			t.Fatal(err)
		}
		if got != test.want {
			t.Errorf("%#v: got %s, want %s", test.in, got, test.want)
		}
	}
}
