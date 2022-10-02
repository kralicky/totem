//go:build mage

package main

import (
	"github.com/kralicky/ragu"
	"github.com/magefile/mage/mg"
)

var Default = All

func All() {
	mg.SerialDeps(Generate)
}

func Generate() error {
	out, err := ragu.GenerateCode(append(ragu.DefaultGenerators()), "**/*.proto")
	if err != nil {
		return err
	}
	for _, file := range out {
		if err := file.WriteToDisk(); err != nil {
			return err
		}
	}
	return nil
}
