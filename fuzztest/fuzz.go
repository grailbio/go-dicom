package fuzz

import (
	"bytes"

	dicom "github.com/programmingman/go-dicom"
)

func Fuzz(data []byte) int {
	_, _ = dicom.ReadDataSet(bytes.NewBuffer(data), dicom.ReadOptions{})
	return 1
}
