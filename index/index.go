package index

import (
	"errors"
	"strings"

	"fmt"

	"os"

	"github.com/hashicorp/hcl"
	hclast "github.com/hashicorp/hcl/hcl/ast"
	hclparser "github.com/hashicorp/hcl/hcl/parser"
	hcltoken "github.com/hashicorp/hcl/hcl/token"
	"github.com/hashicorp/hil"
	hilast "github.com/hashicorp/hil/ast"
	hilparser "github.com/hashicorp/hil/parser"
)

type UntypedSection struct {
	Name          string
	Location      hcltoken.Pos
	Documentation []string
}

type TypedSection struct {
	Type          string
	Name          string
	Location      hcltoken.Pos
	Documentation []string
}

type ReferenceList struct {
	Name      string
	Type      string
	Path      string
	Locations []hcltoken.Pos
}

type Error struct {
	Message  string
	Location hcltoken.Pos
}

type Index struct {
	Version          string
	Errors           []Error
	Variables        []UntypedSection
	DefaultProviders []UntypedSection
	Providers        []TypedSection
	Resources        []TypedSection
	DataResources    []TypedSection
	Modules          []UntypedSection
	Outputs          []UntypedSection
	References       map[string]ReferenceList
	RawAst           *hclast.File
}

const INDEX_VERSION = "1.2.0"

func NewIndex() *Index {
	index := new(Index)
	index.Version = INDEX_VERSION
	index.Errors = []Error{}
	index.Variables = []UntypedSection{}
	index.DefaultProviders = []UntypedSection{}
	index.Providers = []TypedSection{}
	index.Resources = []TypedSection{}
	index.DataResources = []TypedSection{}
	index.Modules = []UntypedSection{}
	index.Outputs = []UntypedSection{}
	index.References = map[string]ReferenceList{}

	index.RawAst = nil
	return index
}

func (index *Index) Collect(astFile *hclast.File, path string, includeRaw bool) error {
	hclast.Walk(astFile.Node, func(current hclast.Node) (hclast.Node, bool) {
		switch current.(type) {
		case *hclast.ObjectList:
			index.handleObjectList(current.(*hclast.ObjectList), path)

		case *hclast.LiteralType:
			index.handleLiteral(current.(*hclast.LiteralType), path)
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

func warning(line, column int, message string) {
	fmt.Fprintf(os.Stderr, "[WARN] %d:%d: %s\n", line, column, message)
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

func commentGroup(group *hclast.CommentGroup) []string {
	comments := []string{}

	if group == nil {
		return comments
	}

	for _, comment := range group.List {
		if comment.Text != "" {
			comments = append(comments, comment.Text)
		}
	}
	return comments
}

func (index *Index) extractResourceDeclaration(item *hclast.ObjectItem, path string) {
	resource := TypedSection{
		Name:          getText(item.Keys[2].Token),
		Type:          getText(item.Keys[1].Token),
		Location:      getPos(item.Keys[2].Token, path), // return position of name
		Documentation: commentGroup(item.LeadComment),
	}
	index.Resources = append(index.Resources, resource)
}

func (index *Index) extractDataDeclaration(item *hclast.ObjectItem, path string) {
	resource := TypedSection{
		Name:          getText(item.Keys[2].Token),
		Type:          getText(item.Keys[1].Token),
		Location:      getPos(item.Keys[2].Token, path), // return position of name
		Documentation: commentGroup(item.LeadComment),
	}
	index.DataResources = append(index.DataResources, resource)
}

func (index *Index) extractVariableDeclaration(item *hclast.ObjectItem, path string) {
	variable := UntypedSection{
		Name:          getText(item.Keys[1].Token),
		Location:      getPos(item.Keys[1].Token, path),
		Documentation: commentGroup(item.LeadComment),
	}
	index.Variables = append(index.Variables, variable)
}

func (index *Index) extractOutputDeclaration(item *hclast.ObjectItem, path string) {
	output := UntypedSection{
		Name:          getText(item.Keys[1].Token),
		Location:      getPos(item.Keys[1].Token, path),
		Documentation: commentGroup(item.LeadComment),
	}
	index.Outputs = append(index.Outputs, output)
}

func (index *Index) extractProviderDeclaration(item *hclast.ObjectItem, path string) {
	// if alias property exists, store as named provider
	alias, err := extractPropertyValue(item, "alias")
	if err != nil {
		provider := UntypedSection{
			Name:          getText(item.Keys[0].Token),
			Location:      getPos(item.Keys[0].Token, path),
			Documentation: commentGroup(item.LeadComment),
		}
		index.DefaultProviders = append(index.DefaultProviders, provider)
	} else {
		provider := TypedSection{
			Name:          alias,
			Location:      getPos(item.Keys[1].Token, path),
			Type:          getText(item.Keys[1].Token),
			Documentation: commentGroup(item.LeadComment),
		}
		index.Providers = append(index.Providers, provider)
	}
}

func (index *Index) extractModuleDeclaration(item *hclast.ObjectItem, path string) {
	module := UntypedSection{
		Name:          getText(item.Keys[1].Token),
		Location:      getPos(item.Keys[1].Token, path),
		Documentation: commentGroup(item.LeadComment),
	}
	index.Modules = append(index.Modules, module)
}

func extractPropertyValue(item *hclast.ObjectItem, property string) (string, error) {
	if item.Val == nil {
		return "", errors.New("Cannot extract property")
	}

	object, ok := item.Val.(*hclast.ObjectType)
	if !ok {
		return "", errors.New("Cannot extract property")
	}

	for _, item := range object.List.Items {
		if len(item.Keys) == 0 {
			continue
		}

		text := getText(item.Keys[0].Token)
		if text != property {
			continue
		}

		if value, ok := item.Val.(*hclast.LiteralType); ok {
			return getText(value.Token), nil
		}
	}

	return "", errors.New("Cannot extract property")
}

func (index *Index) handleObjectList(objectList *hclast.ObjectList, path string) {
	for _, item := range objectList.Items {
		if len(item.Keys) == 0 {
			warning(item.Pos().Line, item.Pos().Column, fmt.Sprintf("Ignoring token with empty 'Keys[]"))
			continue
		}

		firstToken := item.Keys[0].Token
		if firstToken.Type != 4 {
			continue
		}

		switch firstToken.Text {
		case "variable":
			index.extractVariableDeclaration(item, path)
		case "resource":
			index.extractResourceDeclaration(item, path)
		case "data":
			index.extractDataDeclaration(item, path)
		case "provider":
			index.extractProviderDeclaration(item, path)
		case "module":
			index.extractModuleDeclaration(item, path)
		case "output":
			index.extractOutputDeclaration(item, path)
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

func toHclPosWithPath(pos hilast.Pos, path string) hcltoken.Pos {
	result := toHclPos(pos)
	result.Filename = path
	return result
}

func (index *Index) addReference(reference string, pos hcltoken.Pos) {
	parts := strings.SplitN(reference, ".", 3)

	if len(parts) < 2 {
		warning(pos.Line, pos.Column, fmt.Sprintf("Cannot understand reference %s: ", reference))
		return
	}

	name := parts[1]
	list := index.References[name]
	list.Name = name
	if parts[0] == "var" {
		list.Type = "variable"
	} else {
		list.Type = parts[0]
	}
	if len(parts) > 2 {
		list.Path = parts[2]
	}
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

				index.addReference(variable.Name, toHclPosWithPath(variable.Pos(), path))
				break
			}
		}
		return node
	})
}
