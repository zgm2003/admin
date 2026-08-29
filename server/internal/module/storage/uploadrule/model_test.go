package uploadrule

import (
	"reflect"
	"testing"
)

func TestRequestNormalizationAndValidation(t *testing.T) {
	input, err := (createRequest{PlatformID: 1, Code: " avatar ", Name: " Avatar ", CosConfigID: 2, PathPrefix: " avatars ", MaxFileSizeBytes: 1024, MaxFileCount: 2, AllowedExtensions: []string{".PNG", " png ", "JPG"}, AllowedMimeTypes: []string{" Image/PNG ", "image/png"}, AccessMode: "private", IsEnabled: 1, Remark: " avatar files "}).input()
	if err != nil {
		t.Fatal(err)
	}
	if input.Code != "avatar" || input.Name != "Avatar" || input.PathPrefix != "avatars" || input.Remark != "avatar files" {
		t.Fatalf("text normalization = %+v", input)
	}
	if !reflect.DeepEqual(input.AllowedExtensions, []string{"png", "jpg"}) || !reflect.DeepEqual(input.AllowedMimeTypes, []string{"image/png"}) {
		t.Fatalf("array normalization = %#v %#v", input.AllowedExtensions, input.AllowedMimeTypes)
	}

	invalid := []createRequest{
		{PlatformID: 1, Code: "avatar", Name: "Avatar", CosConfigID: 2, PathPrefix: "/", MaxFileSizeBytes: 1, MaxFileCount: 1, AllowedExtensions: []string{"png"}, AccessMode: "private", IsEnabled: 1},
		{PlatformID: 1, Code: "avatar", Name: "Avatar", CosConfigID: 2, PathPrefix: "a/../b", MaxFileSizeBytes: 1, MaxFileCount: 1, AllowedExtensions: []string{"png"}, AccessMode: "private", IsEnabled: 1},
		{PlatformID: 1, Code: "avatar", Name: "Avatar", CosConfigID: 2, PathPrefix: "avatars", MaxFileSizeBytes: 5*1024*1024*1024 + 1, MaxFileCount: 1, AllowedExtensions: []string{"png"}, AccessMode: "private", IsEnabled: 1},
		{PlatformID: 1, Code: "avatar", Name: "Avatar", CosConfigID: 2, PathPrefix: "avatars", MaxFileSizeBytes: 1, MaxFileCount: 0, AllowedExtensions: []string{"png"}, AccessMode: "private", IsEnabled: 1},
		{PlatformID: 1, Code: "avatar", Name: "Avatar", CosConfigID: 2, PathPrefix: "avatars", MaxFileSizeBytes: 1, MaxFileCount: 1, AllowedExtensions: nil, AccessMode: "private", IsEnabled: 1},
		{PlatformID: 1, Code: "avatar", Name: "Avatar", CosConfigID: 2, PathPrefix: "avatars", MaxFileSizeBytes: 1, MaxFileCount: 1, AllowedExtensions: []string{"png"}, AccessMode: "shared", IsEnabled: 1},
		{PlatformID: 1, Code: "avatar", Name: "Avatar", CosConfigID: 2, PathPrefix: "avatars", MaxFileSizeBytes: 1, MaxFileCount: 1, AllowedExtensions: []string{"png"}, AccessMode: "private", IsEnabled: 2},
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
