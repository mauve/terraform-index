package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"
	"strings"

	"fmt"

	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
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

type AstDump struct {
	Variables []VariableDeclaration
	Resources []ResourceDeclaration
	Outputs   []OutputDeclaration
	RawAst    *ast.File
}

func GetText(t token.Token) string {
	return strings.Trim(t.Text, "\"")
}

func GetPos(t token.Token, path string) token.Pos {
	location := t.Pos
	location.Filename = path
	return location
}

func CollectDump(astFile *ast.File, path string) (*AstDump, error) {
	dump := new(AstDump)
	dump.Variables = []VariableDeclaration{}
	dump.Resources = []ResourceDeclaration{}
	dump.Outputs = []OutputDeclaration{}
	dump.RawAst = astFile

	objectList, ok := astFile.Node.(*ast.ObjectList)
	if !ok {
		return nil, fmt.Errorf("Root node is not an objectList %v", astFile.Node)
	}

	for _, item := range objectList.Items {
		firstToken := item.Keys[0].Token
		if firstToken.Type != 4 {
			continue
		}

		switch firstToken.Text {
		case "variable":
			{
				variable := VariableDeclaration{
					Name:     GetText(item.Keys[1].Token),
					Location: GetPos(item.Keys[1].Token, path),
				}
				dump.Variables = append(dump.Variables, variable)
				break
			}

		case "resource":
			{
				resource := ResourceDeclaration{
					Name:     GetText(item.Keys[2].Token),
					Type:     GetText(item.Keys[1].Token),
					Location: GetPos(item.Keys[2].Token, path), // return position of name
				}
				dump.Resources = append(dump.Resources, resource)
				break
			}

		case "output":
			{
				output := OutputDeclaration{
					Name:     GetText(item.Keys[1].Token),
					Location: GetPos(item.Keys[1].Token, path),
				}
				dump.Outputs = append(dump.Outputs, output)
				break
			}
		}
	}

	return dump, nil
}

func Contents(path string) ([]byte, error) {
	if path == "-" {
		return ioutil.ReadAll(os.Stdin)
	}

	return ioutil.ReadFile(path)
}

func main() {
	includeRaw := flag.Bool("raw-ast", false, "include the raw ast")
	path := flag.String("file", "-", "file to parse")

	flag.Parse()

	source, err := Contents(*path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot open path '%s'\n", *path)
		os.Exit(1)
	}

	astFile, err := hcl.ParseBytes(source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Cannot parse '%s'\n", err)
		os.Exit(2)
	}

	dump, err := CollectDump(astFile, *path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Could not collect dump '%s'\n", err)
		os.Exit(3)
	}

	if !*includeRaw {
		dump.RawAst = nil
	}

	json, err := json.MarshalIndent(dump, "", "  ")
	if err != nil {
		os.Exit(3)
	}

	os.Stdout.Write(json)
}
