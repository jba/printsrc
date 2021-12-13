// Copyright 2021 by Jonathan Amsterdam. All rights reserved.

//go:build go1.16
// +build go1.16

package printsrc_test

import (
	"fmt"
	"go/format"
	"log"

	"github.com/jba/printsrc"
)

type Student struct {
	Name    string
	ID      int
	GPA     float64
	Classes []Class
}

type Class struct {
	Name string
	Room string
}

func Example() {
	s := Student{
		Name: "Pat A. Gonia",
		ID:   17,
		GPA:  3.8,
		Classes: []Class{
			{"Geometry", "3.14"},
			{"Dance", "123123"},
		},
	}
	p := printsrc.NewPrinter("github.com/jba/printsrc_test")
	str, err := p.Sprint(s)
	if err != nil {
		log.Fatal(err)
	}
	src, err := format.Source([]byte(str))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s\n", src)

	// Output:
	// Student{
	//	Name: "Pat A. Gonia",
	//	ID:   17,
	//	GPA:  3.8,
	//	Classes: []Class{
	//		{
	//			Name: "Geometry",
	//			Room: "3.14",
	//		},
	//		{
	//			Name: "Dance",
	//			Room: "123123",
	//		},
	// 	},
	// }
}
