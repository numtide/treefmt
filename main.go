/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	"os"

	"github.com/numtide/treefmt/cmd"
)

func main() {
	// todo how are exit codes thrown by commands?
	if err := cmd.NewRoot().Execute(); err != nil {
		os.Exit(1)
	}
}
