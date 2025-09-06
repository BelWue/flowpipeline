package main

import (
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.DateTime})
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	docFile, err := os.Create(outputFile)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to create config file at %s", outputFile)
	}
	defer docFile.Close()

	var mdBuilder MdBuilder
	mdBuilder.WriteSection("flowpipeline Configuration and User Guide", func() {
		mdBuilder.WriteParagraph(generatedInfoPreabmle("meta/doc_generator/main.go"))
		mdBuilder.WriteParagraph(multiline(
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
		mdBuilder.WriteParagraph("This overview is structures as follows:")

		segmentTree := buildSegmentTree(rootDir)
		toc := generateToC(segmentTree)
		doc := generateSegmentDoc(segmentTree, mdBuilder.nesting+1)

		mdBuilder.WriteParagraph(summary("Table of Contents", toc))
		mdBuilder.WriteSection("Available Segments", func() {
			mdBuilder.WriteParagraph(doc)
		})
	})

	_, err = docFile.WriteString(mdBuilder.String())
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to write to config file at %s", outputFile)
		return
	}

	log.Info().Msgf("Successfully generated documentation in %s", outputFile)
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
		log.Fatal().Err(err).Msgf("Failed to read directory %s", path)
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

func generateSegmentDoc(tree *SegmentTree, nestingOffset int) string {
	mdBuilder := MdBuilder{nesting: nestingOffset}
	_generateSegmentDoc(tree, &mdBuilder)
	return mdBuilder.String()
}

func _generateSegmentDoc(tree *SegmentTree, mdBuilder *MdBuilder) {
	if tree.Parent == nil {
		for _, child := range tree.Children {
			_generateSegmentDoc(child, mdBuilder)
		}
		return
	}

	mdBuilder.WriteSection(formatTitle(tree), func() {
		if tree.IsSegment {
			fmt.Fprintf(mdBuilder, "_This segment is implemented in %s._", linkFromPath(tree.Path, filepath.Base(tree.Path)))
			mdBuilder.WriteParagraph(extractPackageDoc(tree.Path))
			fieldsDoc := extractConfigStructDoc(tree)
			if fieldsDoc != "" {
				mdBuilder.WriteParagraph(summary("Configuration options", fieldsDoc))
			}
		} else {
			groupReadme := filepath.Join(tree.Path, "README.md")
			data, err := os.ReadFile(groupReadme)
			if err != nil {
				log.Warn().Err(err).Msgf("Failed to read group README at %s", groupReadme)
				mdBuilder.WriteParagraph("_No group documentation found._")
			} else {
				mdBuilder.WriteParagraph(strings.TrimSpace(string(data)))
			}
			for _, child := range tree.Children {
				_generateSegmentDoc(child, mdBuilder)
			}
		}
	})
}

func generatedInfoPreabmle(path string) string {
	commit := envOr("GITHUB_SHA", "HEAD")
	fileLink := linkFromPath(path, path)
	commitLink := linkFromCommit(commit)

	return fmt.Sprintf("_This document was generated from '%s', based on commit '%s'._", fileLink, commitLink)
}

func extractPackageDoc(path string) string {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)

	if err != nil || node.Doc == nil {
		log.Warn().Err(err).Msgf("No documentation found for file %s", path)
		return "_No segment documentation found._"
	}

	return strings.TrimSpace(node.Doc.Text())
}

// TODO: use examples from Type struct https://pkg.go.dev/go/doc@master#Type
func extractConfigStructDoc(tree *SegmentTree) string {
	type FieldDoc struct {
		Name string
		Type string
		Doc  string
	}

	fset := token.NewFileSet()
	files := []*ast.File{expectParse(fset, tree.Path)}
	pkg, err := doc.NewFromFiles(fset, files, "")
	if err != nil {
		log.Warn().Err(err).Msgf("Failed to parse file %s for documentation", tree.Path)
		return ""
	}

	var configType *doc.Type = nil
	for _, typeDecl := range pkg.Types {
		if !strings.EqualFold(typeDecl.Name, unfilenamify(tree.Name)) { // Config struct is named after segment. Skip if not matching
			continue
		}
		configType = typeDecl
		break
	}

	if configType == nil {
		log.Warn().Msgf("No config type found in segment %s", tree.Name)
		return ""
	}

	if configType.Decl.Tok != token.TYPE { // sanity check
		panic(fmt.Sprintf("Found matching config struct with token type %s in segment %s", configType.Decl.Tok, tree.Name))
	}
	if l := len(configType.Decl.Specs); l != 1 { // sanity check
		panic(fmt.Sprintf("Unexpected number of specs. Expected 1, got %d in segment %s", l, tree.Name))
	}

	// Exported elements of config struct: configType -> Decl -> Specs[0] -> Type -> Fields -> List
	fields := expectType[*ast.StructType](
		expectType[*ast.TypeSpec](configType.Decl.Specs[0]).Type, // Specification of the declared config struct
	).Fields.List // List of fields in the type spec

	var fieldDocs []FieldDoc
	for _, field := range fields {
		onCorrectType(field.Type, func(fieldType *ast.Ident) any { // Field has to be an identifier
			if l := len(field.Names); l != 1 { // I don't know when this would be different
				log.Warn().Msgf("Expected exactly one name for field, got %d in segment %s", l, tree.Name)
				return nil
			}

			fieldName := field.Names[0].Name
			typeName := fieldType.Name
			fieldDoc := field.Doc.Text()

			fieldDocs = append(fieldDocs, FieldDoc{fieldName, typeName, fieldDoc})
			return nil
		}, nil)

		onCorrectType(field.Type, func(fieldType *ast.SelectorExpr) any { // We handle base segments manually
			baseTextOutputSegmentFields := []FieldDoc{
				{"File", "*os.File", "Optional output file. If not set, stdout is used."},
			}
			switch fieldType.Sel.Name {
			case "BaseSegment":
			case "BaseFilterSegment":
			case "BaseTextOutputSegment":
				fieldDocs = append(fieldDocs, baseTextOutputSegmentFields...)
			}
			return nil
		}, nil)
	}

	var fieldDocBuilder strings.Builder
	for i, field := range fieldDocs {
		if i != 0 {
			fieldDocBuilder.WriteString("\n")
		}
		fmt.Fprintf(&fieldDocBuilder, "* **%s** _%s_", field.Name, field.Type)
		if field.Doc != "" {
			fmt.Fprintf(&fieldDocBuilder, ": %s", field.Doc)
		}
	}

	return fieldDocBuilder.String()
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
