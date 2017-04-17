package index

import (
	"regexp"
	"strings"

	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/hcl/hcl/parser"
	"github.com/hashicorp/hcl/hcl/token"
)

type VariableDeclaration struct {
	Name     string
	Location token.Pos
}

type ResourceDeclaration struct {
	Type     string
	Name     string
	Location token.Pos
}

type OutputDeclaration struct {
	Name     string
	Location token.Pos
}

type ReferenceList struct {
	Name      string
	Locations []token.Pos
}

type Error struct {
	Message  string
	Location token.Pos
}

type Index struct {
	Errors     []Error
	Variables  []VariableDeclaration
	Resources  []ResourceDeclaration
	Outputs    []OutputDeclaration
	References map[string]ReferenceList
	RawAst     *ast.File
}

func NewIndex() *Index {
	index := new(Index)
	index.Errors = []Error{}
	index.Variables = []VariableDeclaration{}
	index.Resources = []ResourceDeclaration{}
	index.Outputs = []OutputDeclaration{}
	index.References = map[string]ReferenceList{}
	index.RawAst = nil
	return index
}

func (index *Index) Collect(astFile *ast.File, path string, includeRaw bool) error {
	ast.Walk(astFile.Node, func(current ast.Node) (ast.Node, bool) {
		switch current.(type) {
		case *ast.ObjectList:
			{
				index.handleObjectList(current.(*ast.ObjectList), path)
				break
			}

		case *ast.LiteralType:
			{
				index.handleLiteral(current.(*ast.LiteralType), path)
				break
			}
		}

		return current, true
	})

	if includeRaw {
		index.RawAst = astFile
	}
	return nil
}

func (index *Index) CollectString(contents []byte, path string, includeRaw bool) error {
	astFile, err := hcl.ParseBytes(contents)
	if err != nil {
		index.Errors = append(index.Errors, makeError(err, path))
		return err
	}

	return index.Collect(astFile, path, includeRaw)
}

func makeError(err error, path string) Error {
	if posError, ok := err.(*parser.PosError); ok {
		return Error{
			Message:  posError.Err.Error(),
			Location: posError.Pos,
		}
	}

	return Error{
		Message: err.Error(),
	}
}

func getText(t token.Token) string {
	return strings.Trim(t.Text, "\"")
}

func getPos(t token.Token, path string) token.Pos {
	location := t.Pos
	location.Filename = path
	return location
}

func (index *Index) handleObjectList(objectList *ast.ObjectList, path string) {
	for _, item := range objectList.Items {
		firstToken := item.Keys[0].Token
		if firstToken.Type != 4 {
			continue
		}

		switch firstToken.Text {
		case "variable":
			{
				variable := VariableDeclaration{
					Name:     getText(item.Keys[1].Token),
					Location: getPos(item.Keys[1].Token, path),
				}
				index.Variables = append(index.Variables, variable)
				break
			}

		case "resource":
			{
				resource := ResourceDeclaration{
					Name:     getText(item.Keys[2].Token),
					Type:     getText(item.Keys[1].Token),
					Location: getPos(item.Keys[2].Token, path), // return position of name
				}
				index.Resources = append(index.Resources, resource)
				break
			}

		case "output":
			{
				output := OutputDeclaration{
					Name:     getText(item.Keys[1].Token),
					Location: getPos(item.Keys[1].Token, path),
				}
				index.Outputs = append(index.Outputs, output)
				break
			}
		}
	}
}

func literalSubPos(text string, pos token.Pos, start int, path string) token.Pos {
	for index, char := range text {
		if index == start {
			break
		}

		if char == '\n' {
			pos.Column = 1
			pos.Line++
		} else {
			pos.Column++
		}

		pos.Offset++
	}

	pos.Filename = path
	return pos
}

func (index *Index) handleLiteral(literal *ast.LiteralType, path string) {
	re := regexp.MustCompile(`\${(var\..*?)}`)

	matches := re.FindAllStringIndex(literal.Token.Text, -1)
	if matches == nil {
		return
	}

	for _, match := range matches {
		start := match[0] + 2 // ${
		end := match[1] - 1   // }
		name := literal.Token.Text[start+4 : end]

		pos := literalSubPos(literal.Token.Text, literal.Pos(), start, path)

		_, ok := index.References[name]
		if !ok {
			list := ReferenceList{
				Name:      name,
				Locations: []token.Pos{pos},
			}
			index.References[name] = list
		} else {
			list := index.References[name]
			list.Locations = append(list.Locations, pos)
			index.References[name] = list
		}
	}
}
