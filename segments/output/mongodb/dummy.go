// required to prevent build error with CGO_ENABLED=0 due to
// imports github.com/BelWue/flowpipeline/segments/output/mongodb: build constraints exclude all Go files in segments/output/mongodb
package mongodb
