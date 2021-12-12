// Copyright 2021 by Jonathan Amsterdam. All rights reserved.

/*
Package printsrc prints Go values as Go source.
It strives to render legal Go source code, and returns
an error when detects that it cannot.

To generate code for a slice of Points in package "geo":

   package main

   import "my/packages/geo"

   func main() {
       p := NewPrinter("my/packages/geo")
       data := []geo.Point{{1, 2}, {3, 4}}
       if err := p.Fprint(out, data); err != nil {
           log.Fatal(err)
       }
   }

Registering Import Path Identifiers

To print the names a type in another package, printsrc needs to know how to
refer to the package. Usually, but not always, the package identifier is the
last component of the package path. For example, the types in the standard
library package "database/sql" are normally prefixed by "sql". But this rule
doesn't hold for all packages. The actual identifier is the package name as
declared in its files' package clause, which need not be the same as the last
component of the import path. (A common case: import paths ending in "/v2",
"/v3" and so on.) Also, an import statement can specify a different identifier.
Since printsrc can't know about these special cases, you must call
Printer.RegisterImport to tell it the identifier to use for a given import path.


Registering Custom Printers

Sometimes there is no way for printsrc to discover how to print a value as valid
Go source code. For example, the math/big.Int type is a struct with no exported
fields, so a big.Int cannot be printed as a struct literal. (A big.Int can be
constructed with the NewInt function or the SetString method.)

Use Printer.RegisterPrinter to associate a type with a function that returns
source code for a value of that type.

A custom printer for time.Time is registered by default. It prints a time.Time
by printing a call to time.Date. An error is returned if the time's location is
not Local or UTC, since those are the only locations for which source
expressions can be produced.


Registering Less Functions

This package makes an effort to sort map keys in order to generate deterministic
output. But if it can't sort the keys it prints the map anyway. The output
will be valid Go but the order of the keys will change from run to run.
That creates noise in code review diffs.

Use Printer.RegisterLess to register a function that compares two values
of a type. It will be called to sort map keys of that type.


Known Issues

Maps with multiple NaN keys are not handled.

The reflect package provides no way to distinguish a type defined inside a
function from one at top level. So printsrc will print expressions containing
names for those types which will not compile.

Sharing relationships are not preserved. For example, if two pointers in the input
point to the same value, they will point to different values in the output.

Unexported fields of structs defined outside the generated package are ignored,
because there is no way to set them (without using unsafe code). So important
state may fail to be printed. As a safety feature, printsrc fails if it is asked
to print a non-zero struct from outside the generated package with no exported
fields. You must register custom printers for such structs. But that won't catch
problems with types that have at least one exported field.

Cycles are detected by the crude heuristic of limiting recursion depth. Cycles
cause printsrc to fail. A more sophisticated approach would represent cyclical
values using intermediate variables, but it doesn't seem worth it.
*/
package printsrc
