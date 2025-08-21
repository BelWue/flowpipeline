# Plugin Example

Plugin segments can be written simply as shown in this examples. To get started
with this example:

1. Clone the repo (and check out the desired version tag), this ensures that
   all library versions are in sync
2. run `go build -buildmode=plugin examples/plugin/printcustom.go`
3. have a pipeline use it in a config and call the program with
   `-p printcustom.so` (may be supplied multiple times)
