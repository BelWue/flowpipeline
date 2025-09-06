package main

import (
	"io"
	"strings"
)

type MdBuilder struct {
	io.Writer

	paragraphs []string
	nesting    int
}

func (b *MdBuilder) Write(p []byte) (n int, err error) {
	b.WriteParagraph(string(p))
	return len(p), nil
}

func (b *MdBuilder) WriteParagraph(p string) {
	b.paragraphs = append(b.paragraphs, p)
}

func (b *MdBuilder) WriteSection(heading string, body func()) {
	b.WriteParagraph(strings.Repeat("#", b.nesting+1) + " " + heading)
	b.nesting += 1
	body()
	b.nesting -= 1
}

func (b *MdBuilder) String() string {
	return strings.Join(b.paragraphs, "\n\n")
}
