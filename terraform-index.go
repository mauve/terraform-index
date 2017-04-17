package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"

	"fmt"

	"github.com/mauve/terraform-index/index"
)

const (
	BINARY = "terraform-index"
)

func Contents(path string) ([]byte, error) {
	if path == "-" {
		return ioutil.ReadAll(os.Stdin)
	}

	return ioutil.ReadFile(path)
}

func main() {
	includeRaw := flag.Bool("raw-ast", false, "include the raw ast")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [options] <paths>\n\n", BINARY)
		fmt.Fprintf(os.Stderr, "Extracts references and declarations from Terraform files\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if len(flag.Args()) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	index := index.NewIndex()
	for _, path := range flag.Args() {
		source, err := Contents(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Cannot open path '%s': %s\n", path, err)
			os.Exit(2)
		}

		err = index.CollectString(source, path, *includeRaw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Could not parse '%s': %s\n", path, err)
		}
	}

	json, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		os.Exit(3)
	}

	os.Stdout.Write(json)
}
