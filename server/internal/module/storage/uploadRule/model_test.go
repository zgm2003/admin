package uploadrule

import (
	"reflect"
	"testing"
)

func TestRequestNormalizationAndValidation(t *testing.T) {
	input, err := (createRequest{PlatformID: 1, Codes: []string{" avatar ", "article-cover", "avatar"}, Name: " Avatar ", CosConfigID: 2, MaxFileSizeBytes: 1024, AllowedExtensions: []string{".PNG", " png ", "JPG"}, AllowedMimeTypes: []string{" Image/PNG ", "image/png"}, AccessMode: "private", IsEnabled: 1, Remark: " avatar files "}).input()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(input.Codes, []string{"avatar", "article-cover"}) || input.Name != "Avatar" || input.Remark != "avatar files" {
		t.Fatalf("text normalization = %+v", input)
	}
	if !reflect.DeepEqual(input.AllowedExtensions, []string{"png", "jpg"}) || !reflect.DeepEqual(input.AllowedMimeTypes, []string{"image/png"}) {
		t.Fatalf("array normalization = %#v %#v", input.AllowedExtensions, input.AllowedMimeTypes)
	}

	invalid := []createRequest{
		{PlatformID: 1, Codes: []string{"/avatar"}, Name: "Avatar", CosConfigID: 2, MaxFileSizeBytes: 1, AllowedExtensions: []string{"png"}, AccessMode: "private", IsEnabled: 1},
		{PlatformID: 1, Codes: []string{"a/../b"}, Name: "Avatar", CosConfigID: 2, MaxFileSizeBytes: 1, AllowedExtensions: []string{"png"}, AccessMode: "private", IsEnabled: 1},
		{PlatformID: 1, Codes: []string{"avatar"}, Name: "Avatar", CosConfigID: 2, MaxFileSizeBytes: 5*1024*1024*1024 + 1, AllowedExtensions: []string{"png"}, AccessMode: "private", IsEnabled: 1},
		{PlatformID: 1, Codes: nil, Name: "Avatar", CosConfigID: 2, MaxFileSizeBytes: 1, AllowedExtensions: []string{"png"}, AccessMode: "private", IsEnabled: 1},
		{PlatformID: 1, Codes: []string{"avatar"}, Name: "Avatar", CosConfigID: 2, MaxFileSizeBytes: 1, AllowedExtensions: []string{"png"}, AccessMode: "shared", IsEnabled: 1},
		{PlatformID: 1, Codes: []string{"avatar"}, Name: "Avatar", CosConfigID: 2, MaxFileSizeBytes: 1, AllowedExtensions: []string{"png"}, AccessMode: "private", IsEnabled: 2},
	}
	for index, request := range invalid {
		if _, err := request.input(); err == nil {
			t.Fatalf("invalid request %d accepted: %+v", index, request)
		}
	}
}

func TestStringArrayRoundTripSimpleValues(t *testing.T) {
	want := StringArray{"png", "image/png"}
	encoded, err := want.Value()
	if err != nil {
		t.Fatal(err)
	}
	var got StringArray
	if err := got.Scan(encoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %#v, want %#v", got, want)
	}
}
