package index

import (
	"strings"

	"github.com/hashicorp/hcl"
	hclast "github.com/hashicorp/hcl/hcl/ast"
	hclparser "github.com/hashicorp/hcl/hcl/parser"
	hcltoken "github.com/hashicorp/hcl/hcl/token"
	"github.com/hashicorp/hil"
	hilast "github.com/hashicorp/hil/ast"
	hilparser "github.com/hashicorp/hil/parser"
)

type VariableDeclaration struct {
	Type     string
	Name     string
	Location hcltoken.Pos
}

type ResourceDeclaration struct {
	Type     string
	Name     string
	Location hcltoken.Pos
}

type OutputDeclaration struct {
	Name     string
	Location hcltoken.Pos
}

type ReferenceList struct {
	Name      string
	Locations []hcltoken.Pos
}

type Error struct {
	Message  string
	Location hcltoken.Pos
}

type Index struct {
	Version    string
	Errors     []Error
	Variables  []VariableDeclaration
	Resources  []ResourceDeclaration
	Outputs    []OutputDeclaration
	References map[string]ReferenceList
	RawAst     *hclast.File
}

const INDEX_VERSION = "1.1.0"

func NewIndex() *Index {
	index := new(Index)
	index.Version = INDEX_VERSION
	index.Errors = []Error{}
	index.Variables = []VariableDeclaration{}
	index.Resources = []ResourceDeclaration{}
	index.Outputs = []OutputDeclaration{}
	index.References = map[string]ReferenceList{}
	index.RawAst = nil
	return index
}

func (index *Index) Collect(astFile *hclast.File, path string, includeRaw bool) error {
	hclast.Walk(astFile.Node, func(current hclast.Node) (hclast.Node, bool) {
		switch current.(type) {
		case *hclast.ObjectList:
			{
				index.handleObjectList(current.(*hclast.ObjectList), path)
				break
			}

		case *hclast.LiteralType:
			{
				index.handleLiteral(current.(*hclast.LiteralType), path)
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
	if posError, ok := err.(*hclparser.PosError); ok {
		return Error{
			Message:  posError.Err.Error(),
			Location: posError.Pos,
		}
	}

	return Error{
		Message: err.Error(),
	}
}

func getText(t hcltoken.Token) string {
	return strings.Trim(t.Text, "\"")
}

func getPos(t hcltoken.Token, path string) hcltoken.Pos {
	location := t.Pos
	location.Filename = path
	return location
}

func getVariableType(val *hclast.Node) string {
	varType := "undeclared"

	hclast.Walk(*val, func(current hclast.Node) (hclast.Node, bool) {
		switch current.(type) {
		case *hclast.ObjectList:
			for _, item := range current.(*hclast.ObjectList).Items {
				firstToken := item.Keys[0].Token
				switch {
				case firstToken.Type != 4:
					{
						continue
					}
				case firstToken.Text != "type":
					{
						continue
					}
				}
				hclast.Walk(item.Val, func(typeNode hclast.Node) (hclast.Node, bool) {
					switch typeNode.(type) {
					case *hclast.LiteralType:
						varType = getText(typeNode.(*hclast.LiteralType).Token)
					}
					return typeNode, true
				})
			}
		}
		return current, true
	})
	return varType
}

func (index *Index) handleObjectList(objectList *hclast.ObjectList, path string) {
	for _, item := range objectList.Items {
		firstToken := item.Keys[0].Token
		if firstToken.Type != 4 {
			continue
		}

		switch firstToken.Text {
		case "variable":
			{
				variable := VariableDeclaration{
					Type:     getVariableType(&item.Val),
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

func toHilPos(pos hcltoken.Pos) hilast.Pos {
	return hilast.Pos{
		Column:   pos.Column,
		Line:     pos.Line,
		Filename: pos.Filename,
	}
}

func toHclPos(pos hilast.Pos) hcltoken.Pos {
	return hcltoken.Pos{
		Column:   pos.Column,
		Line:     pos.Line,
		Filename: pos.Filename,
		Offset:   0,
	}
}

func (index *Index) addReference(name string, pos hcltoken.Pos) {
	list := index.References[name]
	list.Locations = append(list.Locations, pos)
	index.References[name] = list
}

func (index *Index) handleLiteral(literal *hclast.LiteralType, path string) {
	root, err := hil.ParseWithPosition(literal.Token.Text, toHilPos(literal.Token.Pos))
	if err != nil {
		if parseError, ok := err.(*hilparser.ParseError); ok {
			index.Errors = append(index.Errors, Error{
				Message:  parseError.Message,
				Location: toHclPos(parseError.Pos),
			})
		} else {
			index.Errors = append(index.Errors, Error{
				Message:  err.Error(),
				Location: literal.Token.Pos,
			})
		}
		return
	}

	root.Accept(func(node hilast.Node) hilast.Node {
		switch node.(type) {
		case *hilast.VariableAccess:
			{
				variable := node.(*hilast.VariableAccess)
				// for now ONLY index variables:
				if !strings.HasPrefix(variable.Name, "var.") {
					break
				}

				index.addReference(variable.Name, toHclPos(variable.Pos()))
				break
			}
		}
		return node
	})
}
