package main

import (
	"fmt"
	"log"
	"os"
	"strings"
)

const rootDir = "segments"
const outputFile = "CONFIGURATION_TEST.md"

func main() {
	docFile, err := os.Create(outputFile)
	if err != nil {
		log.Fatalf("Failed to create config file: %v", err)
	}
	defer docFile.Close()

	var docBuilder strings.Builder
	docBuilder.WriteString("# flowpipeline Configuration and User Guide\n\n")

	generatedInfo, err := generatedInfo("meta/doc_generator/main.go")
	if err != nil {
		log.Fatalf("Failed to generate info: %v", err)
		return
	}
	docBuilder.WriteString(generatedInfo)



	_, err = docFile.WriteString(docBuilder.String())
	if err != nil {
		log.Fatalf("Failed to write to documentation file: %v", err)
		return
	}

	fmt.Printf("Successfully generated documentation in %s\n", outputFile)
}

func generatedInfo(path string) (string, error) {
	commit := envOr("GITHUB_SHA", "HEAD")
	fileLink, err := linkFromPath(path)
	if err != nil {
		return "", err
	}
	commitLink := linkFromCommit(commit)

	return fmt.Sprintf("_This document was generated from '%s', based on commit '%s'._", fileLink, commitLink), nil
}
