// Copyright 2021 by Jonathan Amsterdam. All rights reserved.

/*
Package printsrc prints Go values as Go source.
It strives to render legal Go source code, and returns
an error when detects that it cannot.

To generate code for a slice of ints in package "numbers",
write

   p := NewPrinter("numbers")
   if err := p.Fprint(out, []int{1, 2, 3}); err != nil {
      ...
   }

Here the package which will contain the generated source code doesn't matter,
but it is needed to print non-primitive named types.


Registering Imports

XXX


Registering Custom Printers


A custom printer for time.Time is registered by default. It prints a time.Time
by printing a call to time.Date. An error is returned if the time's location is
not Local or UTC, since those are the only locations for which source
expressions can be produced.


Known Issues

Maps with multiple NaN keys are not handled.

printsrc makes an effort to sort map keys in order to generate deterministic
output. But if it can't sort the keys it prints the map anyway. The output
will be valid Go but the order of the keys will change from run to run.
TODO: custom less funcs

printsrc assumes that impw

The reflect package provides no way to distinguish a type defined inside a
function from one at top level. So printsrc will print expressions containing
names for those types which will not compile.

Sharing relationships are not preserved. For example, if two pointers in the input
point to the same value, they will point to different values in the output.

Cycles are detected by the crude heuristic of limiting recursion depth. Cycles
cause printsrc to fail. A more sophisticated system could represent cyclical
values using intermediate variables, but it doesn't seem worth it.

Unexported fields of structs defined outside the generated package are ignored,
because there is no way to set them (without using unsafe code). So important
state may fail to be printed. As a safety feature, printsrc fails if it is asked
to print a non-zero struct from outside the generated package with no exported
fields. You must register custom printers for such structs.


*/
package printsrc
