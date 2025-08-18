package basic

import (
	"database/sql"
	"fmt"
	"io"
	"os"
)

func UnwrappedExternalError() error {
	file, err := os.Open("test.txt")
	if err != nil {
		return err // want "error from external package 'os' should be wrapped"
	}
	defer file.Close()
	return nil
}

func ProperlyWrappedError() error {
	file, err := os.Open("test.txt")
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err) // OK - properly wrapped
	}
	defer file.Close()
	return nil
}

func UsingPercentV() error {
	file, err := os.Open("test.txt")
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err) // want "use %w instead of %v when wrapping errors"
	}
	defer file.Close()
	return nil
}

func SentinelErrorNoWrap() error {
	var r io.Reader
	_, err := r.Read(nil)
	if err == io.EOF {
		return io.EOF // OK - sentinel error doesn't need wrapping
	}
	return err // want "error from external package 'io' should be wrapped"
}

func SqlErrorNoWrap() error {
	var db *sql.DB
	row := db.QueryRow("SELECT 1")
	var result int
	err := row.Scan(&result)
	if err == sql.ErrNoRows {
		return sql.ErrNoRows // OK - sentinel error
	}
	if err != nil {
		return err // want "error from external package 'database/sql' should be wrapped"
	}
	return nil
}

func InternalError() error {
	err := internalHelper()
	if err != nil {
		return err // OK - same package error
	}
	return nil
}

func internalHelper() error {
	return fmt.Errorf("internal error")
}

func PassThroughError() error {
	file, err := os.Open("test.txt")
	if err != nil {
		logError(err)
		return err // want "error from external package 'os' should be wrapped"
	}
	defer file.Close()
	return nil
}

func logError(err error) {
	fmt.Printf("Error: %v\n", err)
}

func MissingContext() error {
	file, err := os.Open("test.txt")
	if err != nil {
		return fmt.Errorf("%w", err) // OK - context requirement is disabled by default
	}
	defer file.Close()
	return nil
}