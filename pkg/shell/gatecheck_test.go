package shell

import (
	"reflect"
	"testing"
)

func TestAppendGatecheckBuildContextArgs(t *testing.T) {
	o := newOptions(WithBundleBuildContext(
		"pipeline-123",
		"registry.example.com/team/api",
		[]string{"registry.example.com/team/api", "registry.example.com/team/worker"},
	))

	got := appendGatecheckBuildContextArgs([]string{"bundle", "add"}, o)
	want := []string{
		"bundle", "add",
		"--build-group-id", "pipeline-123",
		"--image-name", "registry.example.com/team/api",
		"--build-image-name", "registry.example.com/team/api",
		"--build-image-name", "registry.example.com/team/worker",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestAppendGatecheckBuildContextArgs_OmitsEmptyContext(t *testing.T) {
	args := []string{"bundle", "add"}
	got := appendGatecheckBuildContextArgs(args, newOptions())
	if !reflect.DeepEqual(got, args) {
		t.Fatalf("want %q, got %q", args, got)
	}
}
