package dicom_test

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	dicom "github.com/programmingman/go-dicom"
	"github.com/programmingman/go-dicom/dicomtag"
	"github.com/programmingman/go-dicom/dicomuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustReadFile(path string, options dicom.ReadOptions) *dicom.DataSet {
	data, err := dicom.ReadDataSetFromFile(path, options)
	if err != nil {
		log.Panic(err)
	}
	return data
}

func TestAllFiles(t *testing.T) {
	dir, err := os.Open("examples")
	require.NoError(t, err)
	names, err := dir.Readdirnames(0)
	require.NoError(t, err)
	for _, name := range names {
		t.Logf("Reading %s", name)
		_ = mustReadFile("examples/"+name, dicom.ReadOptions{})
	}
}

func replacePrivateTagVR(elem *dicom.Element) {
	if dicomtag.IsPrivate(elem.Tag.Group) {
		elem.VR = "UN"
	}
	for _, v := range elem.Value {
		if elem, ok := v.(*dicom.Element); ok {
			replacePrivateTagVR(elem)
		}
	}
}

func testWriteFile(t *testing.T, dcmPath, transferSyntaxUID string) {
	data := mustReadFile(dcmPath, dicom.ReadOptions{})
	dstPath := "/tmp/writetest-" + transferSyntaxUID + "-" + filepath.Base(dcmPath)
	t.Logf("Write test %s (transfersyntax %s) -> %s", dcmPath, transferSyntaxUID, dstPath)
	out, err := os.Create(dstPath)
	require.NoError(t, err)

	for i := range data.Elements {
		if data.Elements[i].Tag == dicomtag.TransferSyntaxUID {
			newElem := dicom.MustNewElement(dicomtag.TransferSyntaxUID, transferSyntaxUID)
			t.Logf("Setting transfer syntax UID from %v to %v",
				data.Elements[i].MustGetString(), newElem.MustGetString())
			data.Elements[i] = newElem
		}
	}
	err = dicom.WriteDataSet(out, data)
	require.NoError(t, err)
	data2 := mustReadFile(dstPath, dicom.ReadOptions{})

	if len(data.Elements) != len(data2.Elements) {
		t.Errorf("Wrong # of elements: %v %v", len(data.Elements), len(data2.Elements))
		for _, elem := range data.Elements {
			if _, err := data2.FindElementByTag(elem.Tag); err != nil {
				t.Errorf("Tag %v found in org, but not in new", dicomtag.DebugString(elem.Tag))
			}
		}
		for _, elem := range data2.Elements {
			if _, err := data.FindElementByTag(elem.Tag); err != nil {
				t.Errorf("Tag %v found in new, but not in org", dicomtag.DebugString(elem.Tag))
			}
		}
	}

	if transferSyntaxUID == dicomuid.ImplicitVRLittleEndian {
		// For implicit encoding, VR of private tags will be replaced with UN.
		for _, elem := range data.Elements {
			replacePrivateTagVR(elem)
		}
	}
	for _, elem := range data.Elements {
		elem2, err := data2.FindElementByTag(elem.Tag)
		if err != nil {
			t.Error(err)
			continue
		}
		if elem.Tag == dicomtag.FileMetaInformationGroupLength {
			// This element is expected to change when the file is transcoded.
			continue
		}
		require.Equal(t, elem.String(), elem2.String())
	}
}

func TestWriteFile(t *testing.T) {
	path := "examples/CT-MONO2-16-ort.dcm"
	testWriteFile(t, path, dicomuid.ImplicitVRLittleEndian)
	testWriteFile(t, path, dicomuid.ExplicitVRBigEndian)
	testWriteFile(t, path, dicomuid.ExplicitVRLittleEndian)

	path = "examples/OF_DICOM.dcm"
	testWriteFile(t, path, dicomuid.ImplicitVRLittleEndian)
	testWriteFile(t, path, dicomuid.ExplicitVRBigEndian)
	testWriteFile(t, path, dicomuid.ExplicitVRLittleEndian)
}

func TestReadDataSet(t *testing.T) {
	data := mustReadFile("examples/IM-0001-0001.dcm", dicom.ReadOptions{})
	elem, err := data.FindElementByName("PatientName")
	require.NoError(t, err)
	assert.Equal(t, elem.MustGetString(), "TOUTATIX")
	elem, err = data.FindElementByName("TransferSyntaxUID")
	require.NoError(t, err)
	assert.Equal(t, elem.MustGetString(), "1.2.840.10008.1.2.4.91")
	assert.Equal(t, len(data.Elements), 98)
	_, err = data.FindElementByTag(dicomtag.PixelData)
	assert.NoError(t, err)
}

// Test ReadOptions
func TestReadOptions(t *testing.T) {
	// Test Drop Pixel Data
	data := mustReadFile("examples/IM-0001-0001.dcm", dicom.ReadOptions{DropPixelData: true})
	_, err := data.FindElementByTag(dicomtag.PatientName)
	require.NoError(t, err)
	_, err = data.FindElementByTag(dicomtag.PixelData)
	require.Error(t, err)

	// Test Return Tags
	data = mustReadFile("examples/IM-0001-0001.dcm", dicom.ReadOptions{DropPixelData: true, ReturnTags: []dicomtag.Tag{dicomtag.StudyInstanceUID}})
	_, err = data.FindElementByTag(dicomtag.StudyInstanceUID)
	if err != nil {
		t.Error(err)
	}
	_, err = data.FindElementByTag(dicomtag.PatientName)
	if err == nil {
		t.Errorf("PatientName should not be present")
	}

	// Test Stop at Tag
	data = mustReadFile("examples/IM-0001-0001.dcm",
		dicom.ReadOptions{
			DropPixelData: true,
			// Study Instance UID Element tag is Tag{0x0020, 0x000D}
			StopAtTag: &dicomtag.StudyInstanceUID})
	_, err = data.FindElementByTag(dicomtag.PatientName) // Patient Name Element tag is Tag{0x0010, 0x0010}
	if err != nil {
		t.Error(err)
	}
	_, err = data.FindElementByTag(dicomtag.SeriesInstanceUID) // Series Instance UID Element tag is Tag{0x0020, 0x000E}
	if err == nil {
		t.Errorf("PatientName should not be present")
	}
}

func Example_read() {
	ds, err := dicom.ReadDataSetFromFile("examples/IM-0001-0003.dcm", dicom.ReadOptions{})
	if err != nil {
		panic(err)
	}
	patientID, err := ds.FindElementByTag(dicomtag.PatientID)
	if err != nil {
		panic(err)
	}
	patientBirthDate, err := ds.FindElementByTag(dicomtag.PatientBirthDate)
	if err != nil {
		panic(err)
	}
	fmt.Println("ID: " + patientID.String())
	fmt.Println("BirthDate: " + patientBirthDate.String())
	// Output:
	// ID:  (0010,0020)[PatientID] LO  [7DkT2Tp]
	// BirthDate:  (0010,0030)[PatientBirthDate] DA  [19530828]
}

func Example_updateExistingFile() {
	ds, err := dicom.ReadDataSetFromFile("examples/IM-0001-0003.dcm", dicom.ReadOptions{})
	if err != nil {
		panic(err)
	}
	patientID, err := ds.FindElementByTag(dicomtag.PatientID)
	if err != nil {
		panic(err)
	}
	patientID.Value = []interface{}{"John Doe"}

	buf := bytes.Buffer{}
	if err := dicom.WriteDataSet(&buf, ds); err != nil {
		panic(err)
	}

	ds2, err := dicom.ReadDataSet(&buf, dicom.ReadOptions{})
	if err != nil {
		panic(err)
	}
	patientID, err = ds2.FindElementByTag(dicomtag.PatientID)
	if err != nil {
		panic(err)
	}
	fmt.Println("ID: " + patientID.String())
	// Output:
	// ID:  (0010,0020)[PatientID] LO  [John Doe]
}

func Example_write() {
	elems := []*dicom.Element{
		dicom.MustNewElement(dicomtag.TransferSyntaxUID, dicomuid.ExplicitVRLittleEndian),
		dicom.MustNewElement(dicomtag.MediaStorageSOPClassUID, "1.2.840.10008.5.1.4.1.1.1.2"),
		dicom.MustNewElement(dicomtag.MediaStorageSOPInstanceUID, "1.2.840.113857.113857.1528.141452.1.5"),
		dicom.MustNewElement(dicomtag.PatientName, "Alice Doe"),
	}
	ds := dicom.DataSet{Elements: elems}
	err := dicom.WriteDataSetToFile("/tmp/test.dcm", &ds)
	if err != nil {
		panic(err)
	}

	ds2, err := dicom.ReadDataSetFromFile("/tmp/test.dcm", dicom.ReadOptions{})
	if err != nil {
		panic(err)
	}
	for _, elem := range ds2.Elements {
		fmt.Println(elem.String())
	}
	// Output:
	// (0002,0000)[FileMetaInformationGroupLength] UL  [184]
	//  (0002,0001)[FileMetaInformationVersion] OB  [[48 32 49 0]]
	//  (0002,0002)[MediaStorageSOPClassUID] UI  [1.2.840.10008.5.1.4.1.1.1.2]
	//  (0002,0003)[MediaStorageSOPInstanceUID] UI  [1.2.840.113857.113857.1528.141452.1.5]
	//  (0002,0010)[TransferSyntaxUID] UI  [1.2.840.10008.1.2.1]
	//  (0002,0012)[ImplementationClassUID] UI  [1.2.826.0.1.3680043.9.7133.1.1]
	//  (0002,0013)[ImplementationVersionName] SH  [GODICOM_1_1]
	//  (0010,0010)[PatientName] PN  [Alice Doe]
}

func BenchmarkParseSingle(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = mustReadFile("examples/IM-0001-0001.dcm", dicom.ReadOptions{})
	}
}
