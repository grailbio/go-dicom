package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"

	"github.com/grailbio/go-dicom"
	"github.com/grailbio/go-dicom/dicomtag"
)

var (
	printMetadata = flag.Bool("print-metadata", true, "Print image metadata")
	extractImages = flag.Bool("extract-images", false, "Extract images into separate files")
)

func main() {
	flag.Parse()
	if len(flag.Args()) == 0 {
		log.Panic("dicomutil <dicomfile>")
	}
	path := flag.Arg(0)
	data, err := dicom.ReadDataSetFromFile(path, dicom.ReadOptions{})
	if err != nil {
		panic(err)
	}
	if *printMetadata {
		for _, elem := range data.Elements {
			fmt.Printf("%v\n", elem.String())
		}
	}
	if *extractImages {
		n := 0
		for _, elem := range data.Elements {
			if elem.Tag == dicomtag.PixelData {
				data := elem.Value[0].(dicom.PixelDataInfo).Frames[0]
				imgType := getFormat(bytes.NewBuffer(data))
				if imgType == "" {
					imgType = "jpg" // set default format
				}
				path := fmt.Sprintf("image.%d.%s", n, imgType)
				n++
				ioutil.WriteFile(path, data, 0644)
				fmt.Printf("%s: %d bytes\n", path, len(data))
			}
		}
	}
}

func getFormat(file io.Reader) string {
	bytes := make([]byte, 4)
	n, _ := file.Read(bytes)
	if n < 4 {
		return ""
	}
	if bytes[0] == 0x89 && bytes[1] == 0x50 && bytes[2] == 0x4E && bytes[3] == 0x47 {
		return "png"
	}
	if bytes[0] == 0xFF && bytes[1] == 0xD8 {
		return "jpg"
	}
	if bytes[0] == 0x47 && bytes[1] == 0x49 && bytes[2] == 0x46 && bytes[3] == 0x38 {
		return "gif"
	}
	if bytes[0] == 0x42 && bytes[1] == 0x4D {
		return "bmp"
	}
	return ""
}
