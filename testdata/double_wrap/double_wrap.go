package doublewrap

import (
	"fmt"
	"os"
)

func DoubleWrapSameFunction() error {
	file, err := os.Open("test.txt")
	if err != nil {
		wrappedErr := fmt.Errorf("failed to open: %w", err)
		return fmt.Errorf("operation failed: %w", wrappedErr) // want "error is already wrapped"
	}
	defer file.Close()
	return nil
}

func DoubleWrapAcrossFunctions() error {
	err := helper()
	if err != nil {
		return fmt.Errorf("helper failed: %w", err) // want "error is already wrapped" (if helper already wraps)
	}
	return nil
}

func helper() error {
	file, err := os.Open("test.txt")
	if err != nil {
		return fmt.Errorf("could not open file: %w", err)
	}
	defer file.Close()
	return nil
}

func MultipleWrapping() error {
	file, err := os.Open("test.txt")
	if err != nil {
		err = fmt.Errorf("step 1: %w", err)
		err = fmt.Errorf("step 2: %w", err)        // want "error is already wrapped"
		return fmt.Errorf("final step: %w", err)   // want "error is already wrapped"
	}
	defer file.Close()
	return nil
}

func WrapThenPassThrough() error {
	file, err := os.Open("test.txt")
	if err != nil {
		wrappedErr := fmt.Errorf("failed to open: %w", err)
		if shouldLog() {
			logError(wrappedErr)
		}
		return wrappedErr // OK - already wrapped
	}
	defer file.Close()
	return nil
}

func shouldLog() bool {
	return true
}

func logError(err error) {
	fmt.Printf("Error: %v\n", err)
}