# printsrc: Printing Go Values as Source

There are many packages that print Go values so people can read them.
This package prints Go values so the Go compiler can read them.
It is intended for use in code generators.

## Background

I wanted to provide some data to my program at startup. I could have used
`go:embed` to store the raw data with the program and process it at startup,
but I wanted to pay the processing cost beforehand and generate Go data
structures into a `.go` file that could be linked with the rest of my code.

So I need something that printed Go values as Go source. I looked around at the
many pretty-printing packages out there:

- github.com/davecgh/go-spew/spew
- github.com/k0kubun/pp/v3
- github.com/kr/pretty
- github.com/kylelemons/godebug/pretty
- github.com/sanity-io/litter

and more. They do a great job of formatting Go values for people to read. But I
couldn't find one that correctly prints values as Go source. So I wrote this
package.

## Issues with Printing Source

Here are a few challenges that come up when trying to print Go values in a way
that the compiler can understand.

### Special floating-point values

Consider the floating-point value for positive infinity. There is no Go literal
for this value, but it can be obtained with `math.Inf(1)`. Calling
`fmt.Sprintf("%#v", math.Inf(1))` returns `+Inf`, which is not valid Go.

The `printsrc` package prints a `float64` positive infinity as `math.Inf(1)`
and a `float32` positive infinity as `float32(math.Inf(1))`.
It handles negative infinity and NaN similarly.

### Values that cannot be represented

Function and channel values cannot be written as source using information
available from the `reflect` package. Pretty-printers do their best to render
these values, as they should, but `printsrc` fails on them so you can discover
the problem quickly.


### Pointers

When faced with a pointer, printers either print the address (like Go's `fmt`
package) or follow the pointer and print the value. Neither of those, when fed
back into Go, will produce the right value. Given

```
i := 5
s := []*int{&i, &i}
```
this package will print `s` as
```
[]*int{
    func() *int { var x int = 5; return &x }(),
    func() *int { var x int = 5; return &x }(),
}
```

That is a valid Go expression, although it doesn't preserve the sharing
relationship of the original. For simplicity, `printsrc` doesn't detect sharing,
and fails on cycles.

### Types from other packages

Say your data structure contains a `time.Duration`. Depending on where it
occurs, such values may have to be rendered with their type, like
`time.Duration(100)`. But that code won't compile unless the `time` package has
been imported (and imported under the name "time"). Types in the package for
where the generated code lives don't have that problem; they can be generated
without a qualifying package identifier.

`printsrc` assumes that packages it encounters have been imported using the
identifier that is the last component of their import path. Most of the time
that is correct, but when it isn't you can register a different identifier
with an import path.

### Values that need constructors

The `time.Time` type has unexported fields, so it can't be usably printed as a Go
struct literal (unless the code is being generated in the `time` package itself).
There are many other types that need to be constructed with a function call or
in some other way. Since `printsrc` can't discover the constructors for these
types on its own, it lets you provide custom printing functions for any type.
The one for `time.Time` is built in and prints a call to `time.Date`. (You can
override it if you want.)
