//go:build mage

package main

import (
	"os"
	"path/filepath"

	"github.com/kralicky/ragu/pkg/ragu"
	"github.com/magefile/mage/mg"
)

var Default = All

func All() {
	mg.SerialDeps(Generate)
}

func Generate() error {
	for _, proto := range []string{
		"totem.proto",
		"test/test.proto",
		"examples/simple/example.proto",
	} {
		dir := filepath.Dir(proto)
		protos, err := ragu.GenerateCode(proto)
		if err != nil {
			return err
		}
		for _, f := range protos {
			path := filepath.Join(dir, f.GetName())
			if info, err := os.Stat(path); err == nil {
				if info.Mode()&0200 == 0 {
					if err := os.Chmod(path, 0644); err != nil {
						return err
					}
				}
			}
			if err := os.WriteFile(path, []byte(f.GetContent()), 0444); err != nil {
				return err
			}
			if err := os.Chmod(path, 0444); err != nil {
				return err
			}
		}
	}
	return nil
}
