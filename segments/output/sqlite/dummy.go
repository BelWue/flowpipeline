// required to prevent build error with CGO_ENABLED=0 due to
// imports github.com/BelWue/flowpipeline/segments/output/sqlite: build constraints exclude all Go files in segments/output/sqlite
package sqlite
