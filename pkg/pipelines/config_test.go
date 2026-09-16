package pipelines

import (
	"reflect"
	"testing"

	"github.com/spf13/viper"
)

func TestBuildContextConfigFromEnvironment(t *testing.T) {
	t.Setenv("PORTAGE_BUILD_GROUP_ID", "pipeline-123")
	t.Setenv("PORTAGE_IMAGE_NAME", "registry.example.com/team/api")
	t.Setenv("PORTAGE_BUILD_IMAGE_NAMES", "registry.example.com/team/api,registry.example.com/team/worker")

	v := viper.New()
	BindViper(v)
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		t.Fatal(err)
	}

	if config.BuildGroupID != "pipeline-123" {
		t.Fatalf("want build group ID pipeline-123, got %q", config.BuildGroupID)
	}
	if config.ImageName != "registry.example.com/team/api" {
		t.Fatalf("want API image name, got %q", config.ImageName)
	}
	wantImages := []string{"registry.example.com/team/api", "registry.example.com/team/worker"}
	if !reflect.DeepEqual(config.BuildImageNames, wantImages) {
		t.Fatalf("want build image names %q, got %q", wantImages, config.BuildImageNames)
	}
}
