//go:build mage

package main

import (
	"github.com/kralicky/protols/sdk/codegen"
	"github.com/magefile/mage/mg"
)

var Default = All

func All() {
	mg.SerialDeps(Generate)
}

func Generate() error {
	return codegen.GenerateWorkspace()
}
