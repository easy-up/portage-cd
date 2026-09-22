package pipelines

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
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

func TestWriteGithubActionAllSeparatesRuntimeAndActionDefaults(t *testing.T) {
	var output bytes.Buffer
	err := WriteGithubActionAll(
		&output,
		"docker://ghcr.io/easy-up/portage-action:test",
		[]string{"custom:CUSTOM_INPUT:explicit-default:Custom input"},
	)
	if err != nil {
		t.Fatal(err)
	}

	var action map[string]any
	if err := yaml.Unmarshal(output.Bytes(), &action); err != nil {
		t.Fatal(err)
	}
	inputs, ok := action["inputs"].(map[string]any)
	if !ok {
		t.Fatalf("generated action inputs have unexpected type %T", action["inputs"])
	}

	for inputName, rawInput := range inputs {
		if inputName == "custom" {
			continue
		}
		input, ok := rawInput.(map[string]any)
		if !ok {
			t.Fatalf("generated input %q has unexpected type %T", inputName, rawInput)
		}
		if defaultValue, exists := input["default"]; exists {
			t.Errorf("generated input %q must omit runtime default %v", inputName, defaultValue)
		}
	}

	customInput, ok := inputs["custom"].(map[string]any)
	if !ok {
		t.Fatalf("generated custom input has unexpected type %T", inputs["custom"])
	}
	if got := customInput["default"]; got != "explicit-default" {
		t.Errorf("generated custom input default = %v, want explicit-default", got)
	}
}

func TestBindViperPreservesRuntimeDefaults(t *testing.T) {
	v := viper.New()
	BindViper(v)

	if got := v.GetString("config"); got != ".portage.yml" {
		t.Errorf("runtime config default = %q, want .portage.yml", got)
	}
	if !v.GetBool("imagebuild.enabled") {
		t.Error("runtime image build default must remain enabled")
	}
}
