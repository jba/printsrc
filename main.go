package main

import (
	"math"
	"time"

	"github.com/kr/pretty"
	"github.com/sanity-io/litter"
)

type Float float64

var v1 = map[string]Float{
	"a":  3.5,
	"b":  1.2e-5,
	"\t": Float(math.NaN()),
}

// kr/pretty: map[string]main.Float{"\t":NaN, "a":3.5, "b":1.2e-05}

type S struct {
	At  time.Time
	Map map[string]Float
	u   int
}

var v2 = []S{{time.Now(), v1, 3}}

func main() {
	pretty.Println(v2)
	litter.Dump(v2)
	litter.Options{StrictGo: true, HomePackage: "main"}.Dump(v2)
}
