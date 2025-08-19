package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const rootDir = "segments"
const outputFile = "CONFIGURATION_TEST.md"

type SegmentTree struct {
	Name      string
	Path      string
	Depth     int
	IsSegment bool
	Children  []*SegmentTree
	Parent    *SegmentTree
}

func main() {
	docFile, err := os.Create(outputFile)
	if err != nil {
		log.Fatalf("Failed to create config file: %v", err)
	}
	defer docFile.Close()

	var docBuilder strings.Builder
	docBuilder.WriteString("# flowpipeline Configuration and User Guide\n\n")

	generatedInfo := generatedInfoPreabmle("meta/doc_generator/main.go")
	docBuilder.WriteString(generatedInfo + "\n\n")

	docBuilder.WriteString(multiline(
		"Any flowpipeline is configured in a single yaml file which is either located in",
		"the default `config.yml` or specified using the `-c` option when calling the",
		"binary. The config file contains a single list of so-called segments, which",
		"are processing flows in order. Flows represented by",
		"[protobuf messages](https://github.com/bwNetFlow/protobuf/blob/master/flow-messages-enriched.proto)",
		"within the pipeline.",
		"",
		"Usually, the first segment is from the _input_ group, followed by any number of",
		"different segments. Often, flowpipelines end with a segment from the _output_,",
		"_print_, or _export_ groups. All segments, regardless from which group, accept and",
		"forward their input from previous segment to their subsequent segment, i.e.",
		"even input or output segments can be chained to one another or be placed in the",
		"middle of a pipeline.",
	))

	segmentTree := buildSegmentTree(rootDir)

	docBuilder.WriteString("## Table of Contents\n\nThis overview is structures as follows:\n")
	toc := generateToC(segmentTree)
	docBuilder.WriteString(toc + "\n\n")

	docBuilder.WriteString("## Available Segments\n\n")
	doc := generateSegmentDoc(segmentTree)
	docBuilder.WriteString(doc + "\n\n")

	_, err = docFile.WriteString(docBuilder.String())
	if err != nil {
		log.Fatalf("Failed to write to documentation file: %v", err)
		return
	}

	fmt.Printf("Successfully generated documentation in %s\n", outputFile)
}

func buildSegmentTree(path string) *SegmentTree {
	tree := &SegmentTree{
		Name:      "Root",
		Path:      "/",
		Depth:     0,
		IsSegment: false,
		Children:  make([]*SegmentTree, 0),
		Parent:    nil,
	}
	_buildSegmentTree(path, tree)
	return tree
}

func _buildSegmentTree(path string, tree *SegmentTree) {
	entries, err := os.ReadDir(path)
	if err != nil {
		log.Fatalf("Failed to read directory %s: %v", path, err)
		return
	}

	for _, entry := range entries {
		entryName := entry.Name()
		entryNameNoExt := strings.TrimSuffix(entryName, filepath.Ext(entryName))
		entryPath := filepath.Join(path, entryName)

		if entry.IsDir() {
			childTree := &SegmentTree{
				Name:      entryNameNoExt,
				Path:      entryPath,
				Depth:     tree.Depth + 1,
				IsSegment: false,
				Children:  make([]*SegmentTree, 0),
				Parent:    tree,
			}
			tree.Children = append(tree.Children, childTree)
			_buildSegmentTree(entryPath, childTree)
		} else {
			if tree.Parent == nil || tree.Name != entryNameNoExt {
				continue
			}
			tree.IsSegment = true
			tree.Path = entryPath
		}
	}
}

func generateToC(tree *SegmentTree) string {
	var docBuilder strings.Builder
	_generateToC(tree, &docBuilder)
	return docBuilder.String()
}

func _generateToC(tree *SegmentTree, docBuilder *strings.Builder) {
	if tree.Parent != nil {
		formattedTitle := formatTitle(tree)
		fmt.Fprintf(docBuilder, "%s- %s\n", strings.Repeat("  ", tree.Depth-1), linkTo(formattedTitle, "#"+linkifyText(formattedTitle)))
	}

	if !tree.IsSegment {
		for _, child := range tree.Children {
			_generateToC(child, docBuilder)
		}
	}
}

func generateSegmentDoc(tree *SegmentTree) string {
	var docBuilder strings.Builder
	_generateSegmentDoc(tree, &docBuilder)
	return docBuilder.String()
}

func _generateSegmentDoc(tree *SegmentTree, docBuilder *strings.Builder) {
	if tree.Parent != nil {
		headerLevel := strings.Repeat("#", tree.Depth+2)
		fmt.Fprintf(docBuilder, "%s %s\n", headerLevel, formatTitle(tree))
	}

	if tree.IsSegment {
		fmt.Fprintf(docBuilder, "_This segment is implemented in %s._\n\n", linkFromPath(tree.Path, filepath.Base(tree.Path)))
		packageDoc, err := extractPackageDoc(tree.Path)
		if err != nil {
			packageDoc = "_No segment documentation found._"
		}
		docBuilder.WriteString(packageDoc + "\n")
	} else {
		for _, child := range tree.Children {
			_generateSegmentDoc(child, docBuilder)
		}
	}
}

func generatedInfoPreabmle(path string) string {
	commit := envOr("GITHUB_SHA", "HEAD")
	fileLink := linkFromPath(path, path)
	commitLink := linkFromCommit(commit)

	return fmt.Sprintf("_This document was generated from '%s', based on commit '%s'._", fileLink, commitLink)
}

func extractPackageDoc(path string) (string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return "", err
	}

	if node.Doc == nil {
		return "_No segment documentation found._", nil
	}

	return strings.TrimSpace(node.Doc.Text()), nil
}

func formatTitle(tree *SegmentTree) string {
	var title string
	if tree.IsSegment {
		title = formatSegmentName(tree.Name)
	} else {
		title = formatGroupTitle(tree.Name)
	}

	return title
}

func formatGroupTitle(name string) string {
	return fmt.Sprintf("%s Group", cases.Title(language.English).String(name))

}

func formatSegmentName(name string) string {
	return name
}
