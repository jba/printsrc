// Copyright 2021 by Jonathan Amsterdam. All rights reserved.

/*
time.Time and time.Duration

Imports

Cycles

Bugs:

// - NaN map keys? not handling

can't distinguish structs defined inside a function, but those won't work
As a safety feature, `printsrc` fails if it is asked to print a non-zero struct
with no exported fields.
You must register custom printers for such structs.

   sort map keys

*/
package printsrc
