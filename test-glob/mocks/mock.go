package mocks

import (
	"os"
)

func TestMockFunction() error {
	file, err := os.Open("test.txt")
	if err != nil {
		return err // This should be excluded if **/mocks is in exclusion list
	}
	defer file.Close()
	return nil
}