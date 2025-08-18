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

	docBuilder.WriteString("## Available Segments\n\n")
	segmentTree := buildSegmentTree(rootDir)
	buildSegmentDoc(segmentTree, &docBuilder)

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

func buildSegmentDoc(tree *SegmentTree, docBuilder *strings.Builder) {
	if tree.Parent != nil {
		docBuilder.WriteString("\n" + formatTitle(tree, 2) + "\n")
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
			buildSegmentDoc(child, docBuilder)
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

	if node.Doc != nil {
		return strings.TrimSpace(node.Doc.Text()), nil
	}

	return "_No segment documentation found._", nil
}

func formatTitle(tree *SegmentTree, levelOffset int) string {
	headerLevel := strings.Repeat("#", tree.Depth+levelOffset)

	var title string
	if tree.IsSegment {
		title = formatSegmentName(tree.Name)
	} else {
		title = formatGroupTitle(tree.Name)
	}

	return headerLevel + " " + title
}

func formatGroupTitle(name string) string {
	return fmt.Sprintf("%s Group", cases.Title(language.English).String(name))

}

func formatSegmentName(name string) string {
	return name
}
