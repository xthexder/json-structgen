# json-structgen
Generates Go structs from a Json Schema

## Usage
Build using `go build`

Run `./json-structgen struct.schema.json [package] > struct.go`

All `$ref` paths are relative to the input file, nested `$ref`s may break if they aren't in the same folder.
