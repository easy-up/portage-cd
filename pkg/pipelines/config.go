package pipelines

import (
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"reflect"
	"slices"
	"strings"

	"github.com/go-git/go-git/v5"
	"gopkg.in/yaml.v3"

	"github.com/spf13/viper"
)

// Config contains all parameters for the various pipelines
type Config struct {
	Version                 string             `mapstructure:"version"`
	ImageTag                string             `mapstructure:"imageTag"`
	ArtifactDir             string             `mapstructure:"artifactDir"`
	GatecheckBundleFilename string             `mapstructure:"gatecheckBundleFilename"`
	ImageBuild              configImageBuild   `mapstructure:"imageBuild"`
	ImageScan               configImageScan    `mapstructure:"imageScan"`
	CodeScan                configCodeScan     `mapstructure:"codeScan"`
	ImagePublish            configImagePublish `mapstructure:"imagePublish"`
	Deploy                  configDeploy       `mapstructure:"deploy"`
}

type webhookConfig struct {
	Url              string `mapstructure:"url"`
	AuthorizationVar string `mapstructure:"authorizationVar"`
}

type configImageBuild struct {
	Enabled      bool     `mapstructure:"enabled"`
	BuildDir     string   `mapstructure:"buildDir"`
	Dockerfile   string   `mapstructure:"dockerfile"`
	Platform     string   `mapstructure:"platform"`
	Target       string   `mapstructure:"target"`
	CacheTo      string   `mapstructure:"cacheTo"`
	CacheFrom    string   `mapstructure:"cacheFrom"`
	SquashLayers bool     `mapstructure:"squashLayers"`
	Args         []string `mapstructure:"args"`
}

type configImageScan struct {
	Enabled             bool   `mapstructure:"enabled"`
	SyftFilename        string `mapstructure:"syftFilename"`
	GrypeConfigFilename string `mapstructure:"grypeConfigFilename"`
	GrypeFilename       string `mapstructure:"grypeFilename"`
	ClamavFilename      string `mapstructure:"clamavFilename"`
}

type configCodeScan struct {
	Enabled             bool   `mapstructure:"enabled"`
	GitleaksFilename    string `mapstructure:"gitleaksFilename"`
	GitleaksSrcDir      string `mapstructure:"gitleaksSrcDir"`
	SemgrepFilename     string `mapstructure:"semgrepFilename"`
	SemgrepRules        string `mapstructure:"semgrepRules"`
	SemgrepExperimental bool   `mapstructure:"semgrepExperimental"`
	SemgrepSrcDir       string `mapstructure:"semgrepSrcDir"`
	CoverageFile        string `mapstructure:"coverageFile"`
}

type configImagePublish struct {
	Enabled   bool   `mapstructure:"enabled"`
	BundleTag string `mapstructure:"bundleTag"`
}

type configDeploy struct {
	Enabled                 bool            `mapstructure:"enabled"`
	GatecheckConfigFilename string          `mapstructure:"gatecheckConfigFilename"`
	SuccessWebhooks         []webhookConfig `mapstructure:"successWebhooks"`
}

// metaConfigField is used to map viper values to env variables and their associated default values
type metaConfigField struct {
	Key             string
	Env             string
	ActionInputName string
	ActionType      string
	Default         any
	Description     string
}

var metaConfig = []metaConfigField{
	{
		Key:             "config",
		Env:             "PORTAGE_CONFIG",
		ActionInputName: "config_file",
		ActionType:      "String",
		Default:         nil,
		Description:     "The path to a config file to use when executing portage",
	},
	{
		Key:             "imagetag",
		Env:             "PORTAGE_IMAGE_TAG",
		ActionInputName: "tag",
		ActionType:      "String",
		Default:         nil,
		Description:     "The full image tag for the target container image",
	},

	{
		Key:             "artifactdir",
		Env:             "PORTAGE_ARTIFACT_DIR",
		ActionInputName: "artifact_dir",
		ActionType:      "String",
		Default:         nil,
		Description:     "The target directory for all generated artifacts",
	},
	{
		Key:             "gatecheckbundlefilename",
		Env:             "PORTAGE_GATECHECK_BUNDLE_FILENAME",
		ActionInputName: "gatecheck_bundle_filename",
		ActionType:      "String",
		Default:         nil,
		Description:     "The filename for the gatecheck bundle, a validatable archive of security artifacts",
	},

	{
		Key:             "imagebuild.enabled",
		Env:             "PORTAGE_IMAGE_BUILD_ENABLED",
		ActionInputName: "image_build_enabled",
		ActionType:      "Bool",
		Default:         nil,
		Description:     "Enable/Disable the image build pipeline",
	},
	{
		Key:             "imagebuild.builddir",
		Env:             "PORTAGE_IMAGE_BUILD_DIR",
		ActionInputName: "build_dir",
		ActionType:      "String",
		Default:         nil,
		Description:     "The build directory to using during an image build",
	},
	{
		Key:             "imagebuild.dockerfile",
		Env:             "PORTAGE_IMAGE_BUILD_DOCKERFILE",
		ActionInputName: "dockerfile",
		ActionType:      "String",
		Default:         nil,
		Description:     "The Dockerfile/Containerfile to use during an image build",
	},
	{
		Key:             "imagebuild.platform",
		Env:             "PORTAGE_IMAGE_BUILD_PLATFORM",
		ActionInputName: "platform",
		ActionType:      "String",
		Default:         nil,
		Description:     "The target platform for build (e.g., [linux/amd64])",
	},
	{
		Key:             "imagebuild.target",
		Env:             "PORTAGE_IMAGE_BUILD_TARGET",
		ActionInputName: "target",
		ActionType:      "String",
		Default:         nil,
		Description:     "The target build stage to build",
	},
	{
		Key:             "imagebuild.cacheto",
		Env:             "PORTAGE_IMAGE_BUILD_CACHE_TO",
		ActionInputName: "cache_to",
		ActionType:      "String",
		Default:         nil,
		Description:     "Cache export destinations (e.g., \"user/app:cache\", \"type=local,src=path/to/dir\")",
	},
	{
		Key:             "imagebuild.cachefrom",
		Env:             "PORTAGE_IMAGE_BUILD_CACHE_FROM",
		ActionInputName: "cache_from",
		ActionType:      "String",
		Default:         nil,
		Description:     "External cache sources (e.g., \"user/app:cache\", \"type=local,src=path/to/dir\")",
	},
	{
		Key:             "imagebuild.squashlayers",
		Env:             "PORTAGE_IMAGE_BUILD_SQUASH_LAYERS",
		ActionInputName: "squash_layers",
		ActionType:      "Bool",
		Default:         nil,
		Description:     "squash image layers - Only Supported with Podman CLI",
	},
	{
		Key:             "imagebuild.args",
		Env:             "PORTAGE_IMAGE_BUILD_ARGS",
		ActionInputName: "build_args",
		ActionType:      "List",
		Default:         nil,
		Description:     "Comma seperated list of build time variables",
	},
	{
		Key:             "imagescan.enabled",
		Env:             "PORTAGE_IMAGE_SCAN_ENABLED",
		Default:         nil,
		ActionInputName: "image_scan_enabled",
		ActionType:      "Bool",
		Description:     "Enable/Disable the image scan pipeline",
	},
	{
		Key:             "imagescan.syftfilename",
		Env:             "PORTAGE_IMAGE_SCAN_SYFT_FILENAME",
		ActionInputName: "syft_filename",
		ActionType:      "String",
		Default:         nil,
		Description:     "The filename for the syft SBOM report - must contain 'syft'",
	},
	{
		Key:             "imagescan.grypeconfigfilename",
		Env:             "PORTAGE_IMAGE_SCAN_GRYPE_CONFIG_FILENAME",
		ActionInputName: "grype_config_filename",
		ActionType:      "String",
		Default:         nil,
		Description:     "The config filename for the grype vulnerability report",
	},
	{
		Key:             "imagescan.grypefilename",
		Env:             "PORTAGE_IMAGE_SCAN_GRYPE_FILENAME",
		ActionInputName: "grype_filename",
		ActionType:      "String",
		Default:         nil,
		Description:     "The filename for the grype vulnerability report - must contain 'grype'",
	},
	{
		Key:             "imagescan.clamavfilename",
		Env:             "PORTAGE_IMAGE_SCAN_CLAMAV_FILENAME",
		ActionInputName: "clamav_filename",
		ActionType:      "String",
		Default:         nil,
		Description:     "The filename for the clamscan virus report - must contain 'clamav'",
	},

	{
		Key:             "codescan.enabled",
		Env:             "PORTAGE_CODE_SCAN_ENABLED",
		ActionInputName: "code_scan_enabled",
		ActionType:      "Bool",
		Default:         nil,
		Description:     "Enable/Disable the code scan pipeline",
	},
	{
		Key:             "codescan.gitleaksfilename",
		Env:             "PORTAGE_CODE_SCAN_GITLEAKS_FILENAME",
		ActionInputName: "gitleaks_filename",
		ActionType:      "String",
		Default:         nil,
		Description:     "The filename for the gitleaks secret report - must contain 'gitleaks'",
	},
	{
		Key:             "codescan.gitleakssrcdir",
		Env:             "PORTAGE_CODE_SCAN_GITLEAKS_SRC_DIR",
		ActionInputName: "gitleaks_src_dir",
		ActionType:      "String",
		Default:         nil,
		Description:     "The target directory for the gitleaks scan",
	},
	{
		Key:             "codescan.semgrepfilename",
		Env:             "PORTAGE_CODE_SCAN_SEMGREP_FILENAME",
		ActionInputName: "semgrep_filename",
		ActionType:      "String",
		Default:         nil,
		Description:     "The filename for the semgrep SAST report - must contain 'semgrep'",
	},
	{
		Key:             "codescan.semgreprules",
		Env:             "PORTAGE_CODE_SCAN_SEMGREP_RULES",
		ActionInputName: "semgrep_rules",
		ActionType:      "String",
		Default:         nil,
		Description:     "Semgrep ruleset manual override",
	},
	{
		Key:             "codescan.semgrepexperimental",
		Env:             "PORTAGE_CODE_SCAN_SEMGREP_EXPERIMENTAL",
		ActionInputName: "semgrep_experimental",
		ActionType:      "Bool",
		Default:         nil,
		Description:     "Enable the use of the semgrep experimental CLI",
	},
	{
		Key:             "codescan.semgrepsrcdir",
		Env:             "PORTAGE_CODE_SCAN_SEMGREP_SRC_DIR",
		ActionInputName: "semgrep_src_dir",
		ActionType:      "String",
		Default:         nil,
		Description:     "The target directory for the semgrep scan",
	},
	{
		Key:             "codescan.coveragefile",
		Env:             "PORTAGE_CODE_SCAN_COVERAGE_FILE",
		ActionInputName: "coverage_file",
		ActionType:      "String",
		Default:         nil,
		Description:     "An externally generated code coverage file to validate",
	},

	{
		Key:             "imagepublish.enabled",
		Env:             "PORTAGE_IMAGE_PUBLISH_ENABLED",
		ActionInputName: "image_publish_enabled",
		ActionType:      "Bool",
		Default:         nil,
		Description:     "Enable/Disable the image publish pipeline",
	},
	{
		Key:             "imagepublish.bundletag",
		Env:             "PORTAGE_IMAGE_PUBLISH_BUNDLE_TAG",
		ActionInputName: "bundle_publish_tag",
		ActionType:      "String",
		Default:         nil,
		Description:     "The full image tag for the target gatecheck bundle image blob",
	},

	{
		Key:             "deploy.enabled",
		Env:             "PORTAGE_DEPLOY_ENABLED",
		ActionInputName: "deploy_enabled",
		ActionType:      "Bool",
		Default:         nil,
		Description:     "Enable/Disable the deploy pipeline",
	},
	{
		Key:             "deploy.gatecheckconfigfilename",
		Env:             "PORTAGE_DEPLOY_GATECHECK_CONFIG_FILENAME",
		ActionInputName: "gatecheck_config_filename",
		ActionType:      "String",
		Default:         nil,
		Description:     "The filename for the gatecheck config",
	},
}

// Add this near the top of the file with other type definitions
type defaultValues struct {
	value      any
	configPath string
}

// Add this after type definitions but before metaConfig
var defaults = map[string]defaultValues{
	"config":                  {value: ".portage.yml", configPath: ""},
	"imagetag":                {value: "my-app:latest", configPath: "ImageTag"},
	"artifactdir":             {value: "artifacts", configPath: "ArtifactDir"},
	"gatecheckbundlefilename": {value: "gatecheck-bundle.tar.gz", configPath: "GatecheckBundleFilename"},

	"imagebuild.enabled":      {value: true, configPath: "ImageBuild.Enabled"},
	"imagebuild.builddir":     {value: ".", configPath: "ImageBuild.BuildDir"},
	"imagebuild.dockerfile":   {value: "Dockerfile", configPath: "ImageBuild.Dockerfile"},
	"imagebuild.squashlayers": {value: false, configPath: "ImageBuild.SquashLayers"},

	"imagescan.enabled":        {value: true, configPath: "ImageScan.Enabled"},
	"imagescan.syftfilename":   {value: "syft-sbom-report.json", configPath: "ImageScan.SyftFilename"},
	"imagescan.grypefilename":  {value: "grype-vulnerability-report-full.json", configPath: "ImageScan.GrypeFilename"},
	"imagescan.clamavfilename": {value: "clamav-virus-report.txt", configPath: "ImageScan.ClamavFilename"},

	"codescan.enabled":             {value: true, configPath: "CodeScan.Enabled"},
	"codescan.gitleaksfilename":    {value: "gitleaks-secrets-report.json", configPath: "CodeScan.GitleaksFilename"},
	"codescan.gitleakssrcdir":      {value: ".", configPath: "CodeScan.GitleaksSrcDir"},
	"codescan.semgrepfilename":     {value: "semgrep-sast-report.json", configPath: "CodeScan.SemgrepFilename"},
	"codescan.semgreprules":        {value: "p/default", configPath: "CodeScan.SemgrepRules"},
	"codescan.semgrepexperimental": {value: false, configPath: "CodeScan.SemgrepExperimental"},
	"codescan.semgrepsrcdir":       {value: ".", configPath: "CodeScan.SemgrepSrcDir"},

	"imagepublish.enabled": {value: true, configPath: "ImagePublish.Enabled"},

	"deploy.enabled":                 {value: true, configPath: "Deploy.Enabled"},
	"deploy.gatecheckconfigfilename": {value: "", configPath: "Deploy.GatecheckConfigFilename"},
}

// Update metaConfig to use the shared defaults
func init() {
	for idx, field := range metaConfig {
		if defaultVal, exists := defaults[field.Key]; exists {
			metaConfig[idx].Default = defaultVal.value
		}
	}
}

// Add this new function
func NewDefaultConfig() *Config {
	config := &Config{}

	// Use reflection to set the default values
	configValue := reflect.ValueOf(config).Elem()

	for _, defaultVal := range defaults {
		if defaultVal.value == nil {
			continue
		}

		path := strings.Split(defaultVal.configPath, ".")
		current := configValue

		// Navigate nested structs
		for i, field := range path {
			if i == len(path)-1 {
				// Set the final field value
				fieldValue := current.FieldByName(field)
				if fieldValue.IsValid() && fieldValue.CanSet() {
					fieldValue.Set(reflect.ValueOf(defaultVal.value))
				}
			} else {
				// Navigate to nested struct
				current = current.FieldByName(field)
			}
		}
	}

	return config
}

// Add this method to Config struct
func (c *Config) ToMap() map[string]interface{} {
	// Use reflection to convert the config to a map
	configMap := make(map[string]interface{})
	val := reflect.ValueOf(c).Elem()
	typ := val.Type()

	var addToMap func(prefix string, v reflect.Value, t reflect.Type)
	addToMap = func(prefix string, v reflect.Value, t reflect.Type) {
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			value := v.Field(i)

			key := field.Tag.Get("mapstructure")
			if key == "" {
				key = strings.ToLower(field.Name)
			}

			if prefix != "" {
				key = prefix + "." + key
			}

			if value.Kind() == reflect.Struct {
				addToMap(key, value, field.Type)
			} else {
				configMap[key] = value.Interface()
			}
		}
	}

	addToMap("", val, typ)
	return configMap
}

func githubActionsMetaConfig(additionalInputs []string) ([]metaConfigField, error) {
	supportedKeys := []string{
		"config",
		"imagetag",
		"imagebuild.enabled",
		"imagebuild.builddir",
		"imagebuild.dockerfile",
		"imagebuild.platform",
		"imagebuild.target",
		"imagebuild.args",
		"imagescan.enabled",
		"codescan.enabled",
		"codescan.semgreprules",
		"codescan.semgrepsrcdir",
		"codescan.coveragefile",
		"imagepublish.enabled",
		"imagepublish.bundletag",
		"deploy.enabled",
		"deploy.gatecheckconfigfilename",
	}
	fields := make([]metaConfigField, 0)

	for _, field := range metaConfig {
		// filter non-supported fields
		if slices.Contains(supportedKeys, field.Key) {
			fields = append(fields, field)
		}
	}

	for _, additionalInput := range additionalInputs {
		parts := strings.SplitN(additionalInput, ":", 4)
		if len(parts) < 4 {
			return nil, fmt.Errorf("invalid additional input specification: %s", additionalInput)
		}
		fields = append(fields, metaConfigField{
			Key:             parts[0],
			ActionInputName: parts[0],
			Env:             parts[1],
			Default:         parts[2],
			Description:     parts[3],
		})
	}

	return fields, nil
}

func BindViper(v *viper.Viper) {
	for _, field := range metaConfig {
		v.MustBindEnv(field.Key, field.Env)
		if field.Default != nil {
			v.SetDefault(field.Key, field.Default)
		}
	}
}

type githubAction struct {
	Name        string                      `yaml:"name"`
	Description string                      `yaml:"description"`
	Inputs      map[string]actionInputField `yaml:"inputs"`
	Runs        actionRunsConfig            `yaml:"runs"`
}

type actionInputField struct {
	Description string `yaml:"description"`
	Default     string `yaml:"default,omitempty"`
}

type actionRunsConfig struct {
	Using string            `yaml:"using"`
	Image string            `yaml:"image"`
	Args  []string          `yaml:"args,flow"`
	Env   map[string]string `yaml:"env"`
}

func WriteGithubActionAll(dst io.Writer, image string, additionalInputs []string) error {
	action := githubAction{
		Name:        "Portage CD",
		Description: "Code Scan + Image Build + Image Scan + Image Publish + Validation",
		Inputs:      map[string]actionInputField{},
		Runs: actionRunsConfig{
			Using: "docker",
			Image: image,
			Args:  []string{},
			Env:   map[string]string{},
		},
	}

	fields, err := githubActionsMetaConfig(additionalInputs)
	if err != nil {
		return err
	}
	for _, field := range fields {
		// filter non-supported fields
		action.Inputs[field.ActionInputName] = actionInputField{
			Description: field.Description,
			Default:     defaultValueToString(field.Default, ""),
		}
		action.Runs.Env[field.Env] = fmt.Sprintf("${{ inputs.%s }}", field.ActionInputName)
	}

	enc := yaml.NewEncoder(dst)
	enc.SetIndent(2)
	return enc.Encode(action)
}

func RenderTemplate(dst io.Writer, templateSrc io.Reader) error {
	builtins, err := BuiltIns()
	if err != nil {
		return fmt.Errorf("template rendering failed: could not load built-in values: %w", err)
	}
	tmpl := template.New("portage config")

	content, err := io.ReadAll(templateSrc)
	if err != nil {
		return fmt.Errorf("template rendering failed: could not load template content: %w", err)
	}

	tmpl, err = tmpl.Parse(string(content))
	if err != nil {
		return fmt.Errorf("template rendering failed: could not parse template input: %w", err)
	}

	return tmpl.Execute(dst, builtins)
}

func BuiltIns() (map[string]string, error) {
	builtins := map[string]string{}

	slog.Debug("open current repo", "step", "builtins")
	r, err := git.PlainOpen(".")
	if err != nil {
		return builtins, err
	}

	slog.Debug("get repo HEAD")
	ref, err := r.Head()
	if err != nil {
		return builtins, err
	}

	builtins["GitCommitSHA"] = ref.Hash().String()
	builtins["GitCommitShortSHA"] = ref.Hash().String()[:8]
	builtins["GitCommitBranch"] = ref.Name().Short()

	return builtins, nil
}

func defaultValueToString(v any, valueIfNil string) string {
	var defaultValue string
	enabled, isBool := v.(bool)
	switch {
	case v == nil:
		defaultValue = valueIfNil
	case isBool && enabled:
		defaultValue = "1"
	case isBool && !enabled:
		defaultValue = "0"
	default:
		defaultValue = fmt.Sprintf("%v", v)
	}

	return defaultValue
}

func paddedMetaConfigData() [][]string {
	data := [][]string{{"Config Key", "Environment Variable", "Default Value", "Description"}}
	for _, field := range metaConfig {

		newRow := []string{
			field.Key,
			field.Env,
			defaultValueToString(field.Default, "-"),
			field.Description,
		}

		data = append(data, newRow)
	}

	pad(data)

	return data
}

func paddedActionsTable(additionalInputs []string) ([][]string, error) {
	data := [][]string{{"Name", "Type", "Default Value", "Description"}}
	fields, err := githubActionsMetaConfig(additionalInputs)
	if err != nil {
		return nil, err
	}
	for _, field := range fields {

		newRow := []string{
			field.ActionInputName,
			field.ActionType,
			defaultValueToString(field.Default, ""),
			field.Description,
		}

		data = append(data, newRow)
	}

	pad(data)

	return data, nil
}

func pad(data [][]string) {
	maxLengthForCol := make([]int, len(data[0]))

	// find the max length for each field in the slice
	for rowIdx := range data {
		for colIdx := range data[rowIdx] {
			maxLengthForCol[colIdx] = max(len(data[rowIdx][colIdx]), maxLengthForCol[colIdx])
		}
	}

	// Pad each "cell" with spaces based on the max length for the column
	for rowIdx := range data {
		for colIdx := range data[rowIdx] {
			format := fmt.Sprintf("%%-%ds", maxLengthForCol[colIdx])
			data[rowIdx][colIdx] = fmt.Sprintf(format, data[rowIdx][colIdx])
		}
	}
}

func markdownTable(data [][]string) string {
	var sb strings.Builder

	// header row
	row := strings.Join(data[0], " | ")
	sb.WriteString(fmt.Sprintf("| %s |\n", row))

	// header seperator
	seperatorRowData := make([]string, len(data[0]))
	for idx := range seperatorRowData {
		seperatorRowData[idx] = strings.Repeat("-", len(data[0][idx]))
		row = strings.Join(seperatorRowData, " | ")
	}

	sb.WriteString(fmt.Sprintf("| %s |\n", row))

	// Data Rows
	for rowIdx := range data {
		if rowIdx == 0 {
			continue
		}
		row = strings.Join(data[rowIdx], " | ")
		sb.WriteString(fmt.Sprintf("| %s |\n", row))
	}

	return sb.String()
}

func WriteConfigAsMarkdownTable(dst io.Writer) error {
	s := markdownTable(paddedMetaConfigData())
	_, err := strings.NewReader(s).WriteTo(dst)
	return err
}

func WriteConfigAsActionsTable(additionalInputs []string, dst io.Writer) error {
	actionsTable, err := paddedActionsTable(additionalInputs)
	if err != nil {
		return err
	}
	s := markdownTable(actionsTable)
	_, err = strings.NewReader(s).WriteTo(dst)
	return err
}
