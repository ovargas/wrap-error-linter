package main

import (
	"fmt"
	"os"
)

func main() {
	if err := processFile("example.txt"); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func processFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		// This should trigger: error from external package 'os' should be wrapped
		return err
	}
	defer file.Close()

	return nil
}

func processFileCorrect(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		// This is correct - properly wrapped with context
		return fmt.Errorf("failed to open file %s: %w", filename, err)
	}
	defer file.Close()

	return nil
}

func processFileWrongVerb(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		// This should trigger: use %w instead of %v when wrapping errors
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	return nil
}