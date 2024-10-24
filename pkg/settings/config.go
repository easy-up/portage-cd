package settings

import (
	"errors"
	"log/slog"
	"reflect"
)

// Config contains all parameters for the various pipelines
type Config struct {
	Version                 string             `mapstructure:"version"`
	ImageTag                string             `mapstructure:"imageTag"                metafield:"ImageTag"`
	ArtifactDir             string             `mapstructure:"artifactDir"             metafield:"ArtifactDir"`
	GatecheckBundleFilename string             `mapstructure:"gatecheckBundleFilename" metafield:"GatecheckBundleFilename"`
	ImageBuild              configImageBuild   `mapstructure:"imageBuild"`
	ImageScan               configImageScan    `mapstructure:"imageScan"`
	CodeScan                configCodeScan     `mapstructure:"codeScan"`
	ImagePublish            configImagePublish `mapstructure:"imagePublish"`
	Validation              configValidation   `mapstructure:"deploy"`
}

func NewConfig() *Config {
	return new(Config)
}

type configImageBuild struct {
	Enabled      bool   `mapstructure:"enabled"      metafield:"ImageBuildEnabled"`
	BuildDir     string `mapstructure:"buildDir"     metafield:"ImageBuildBuildDir"`
	Dockerfile   string `mapstructure:"dockerfile"   metafield:"ImageBuildDockerfile"`
	Platform     string `mapstructure:"platform"     metafield:"ImageBuildPlatform"`
	Target       string `mapstructure:"target"       metafield:"ImageBuildTarget"`
	CacheTo      string `mapstructure:"cacheTo"      metafield:"ImageBuildCacheTo"`
	CacheFrom    string `mapstructure:"cacheFrom"    metafield:"ImageBuildCacheFrom"`
	SquashLayers bool   `mapstructure:"squashLayers" metafield:"ImageBuildSquashLayers"`
	Args         string `mapstructure:"args"         metafield:"ImageBuildArgs"`
}

type configImageScan struct {
	Enabled             bool   `mapstructure:"enabled"             metafield:"ImageScanEnabled"`
	SyftFilename        string `mapstructure:"syftFilename"        metafield:"ImageScanSyftFilename"`
	GrypeConfigFilename string `mapstructure:"grypeConfigFilename" metafield:"ImageScanGrypeConfigFilename"`
	GrypeFilename       string `mapstructure:"grypeFilename"       metafield:"ImageScanGrypeFilename"`
	ClamavFilename      string `mapstructure:"clamavFilename"      metafield:"ImageScanClamavFilename"`
	FreshclamDisabled   bool   `mapstructure:"freshclamDisabled"   metafield:"ImageScanFreshclamDisabled"`
}

type configCodeScan struct {
	Enabled             bool   `mapstructure:"enabled"             metafield:"CodeScanEnabled"`
	GitleaksFilename    string `mapstructure:"gitleaksFilename"    metafield:"CodeScanGitleaksFilename"`
	GitleaksSrcDir      string `mapstructure:"gitleaksSrcDir"      metafield:"CodeScanGitleaksSrcDir"`
	SemgrepFilename     string `mapstructure:"semgrepFilename"     metafield:"CodeScanSemgrepFilename"`
	SemgrepRules        string `mapstructure:"semgrepRules"        metafield:"CodeScanSemgrepRules"`
	SemgrepExperimental bool   `mapstructure:"semgrepExperimental" metafield:"CodeScanSemgrepExperimental"`
	SemgrepSrcDir       string `mapstructure:"semgrepSrcDir"       metafield:"CodeScanSemgrepSrcDir"`
	SnykFilename        string `mapstructure:"snykFilename"        metafield:"CodeScanSnykFilename"`
	SnykSrcDir          string `mapstructure:"snykSrcDir"          metafield:"CodeScanSnykSrcDir"`
}

type configImagePublish struct {
	Enabled   bool   `mapstructure:"enabled"              metafield:"ImagePublishEnabled"`
	BundleTag string `mapstructure:"bundleTag"            metafield:"ImagePublishBundleTag"`
}

type configValidation struct {
	Enabled                 bool   `mapstructure:"enabled"                 metafield:"ValidationEnabled"`
	GatecheckConfigFilename string `mapstructure:"gatecheckConfigFilename" metafield:"ValidationGatecheckConfigFilename"`
}

type MetaConfig struct {
	ImageTag                          MetaField
	ArtifactDir                       MetaField
	GatecheckBundleFilename           MetaField
	ImageBuildEnabled                 MetaField
	ImageBuildBuildDir                MetaField
	ImageBuildDockerfile              MetaField
	ImageBuildPlatform                MetaField
	ImageBuildTarget                  MetaField
	ImageBuildCacheTo                 MetaField
	ImageBuildCacheFrom               MetaField
	ImageBuildSquashLayers            MetaField
	ImageBuildArgs                    MetaField
	ImageScanEnabled                  MetaField
	ImageScanSyftFilename             MetaField
	ImageScanGrypeConfigFilename      MetaField
	ImageScanGrypeFilename            MetaField
	ImageScanClamavFilename           MetaField
	ImageScanFreshclamDisabled        MetaField
	CodeScanEnabled                   MetaField
	CodeScanGitleaksFilename          MetaField
	CodeScanSemgrepFilename           MetaField
	CodeScanSemgrepRules              MetaField
	CodeScanSemgrepExperimental       MetaField
	CodeScanSemgrepSrcDir             MetaField
	CodeScanGitleaksSrcDir            MetaField
	CodeScanSnykFilename              MetaField
	CodeScanCoverageFile              MetaField
	CodeScanSnykSrcDir                MetaField
	ImagePublishEnabled               MetaField
	ImagePublishBundleTag             MetaField
	ValidationEnabled                 MetaField
	ValidationGatecheckConfigFilename MetaField
}

func Unmarshal(toConfig *Config, fromMetaConfig *MetaConfig) error {
	slog.Debug("start unmarshal")
	if toConfig == nil || fromMetaConfig == nil {
		return errors.New("dst/src is nil")
	}

	toConfigFields := make(map[string]reflect.Value)

	values := []reflect.Value{reflect.ValueOf(toConfig).Elem()}
	for len(values) != 0 {
		// pop operation
		value := values[len(values)-1]
		values = values[:len(values)-1]

		for i := 0; i < value.NumField(); i++ {

			if value.Field(i).Kind() == reflect.Struct {
				values = append(values, value.Field(i))
				continue
			}

			metaFieldStr := value.Type().Field(i).Tag.Get("metafield")
			if metaFieldStr == "" {
				continue
			}
			toConfigFields[metaFieldStr] = value.Field(i)
		}
	}

	metaConfigValue := reflect.ValueOf(fromMetaConfig).Elem()

	for key, toValue := range toConfigFields {

		_, exists := metaConfigValue.Type().FieldByName(key)
		if !exists {
			slog.Error("field not found", "key", key)
			return nil
		}
		field, ok := metaConfigValue.FieldByName(key).Interface().(MetaField)
		if !ok {
			panic("invalid metafield type")
		}

		v := reflect.ValueOf(field.MustEvaluate())
		toValue.Set(v)
	}

	return nil
}

func MustUnmarshal(toConfig *Config, fromMetaConfig *MetaConfig) {
	err := Unmarshal(toConfig, fromMetaConfig)
	if err != nil {
		panic(err)
	}
}

func NewMetaConfig() *MetaConfig {
	m := &MetaConfig{
		ImageTag: MetaField{
			FlagValueP:      new(string),
			FlagName:        "tag",
			FlagDesc:        "The full image tag for the target container image",
			EnvKey:          "PORTAGE_IMAGE_TAG",
			ActionInputName: "tag",
			ActionType:      "String",
			DefaultValue:    "my-app:latest",
			stringDecoder:   stringToStringDecoder,
			cobraFunc:       stringVarCobraFunc,
		},
		ArtifactDir: MetaField{
			FlagValueP:      new(string),
			FlagName:        "artifact-dir",
			FlagDesc:        "The target directory for all generated artifacts",
			EnvKey:          "PORTAGE_ARTIFACT_DIR",
			ActionInputName: "artifact_dir",
			ActionType:      "String",
			DefaultValue:    "artifacts",
			stringDecoder:   stringToStringDecoder,
			cobraFunc:       stringVarCobraFunc,
		},
		GatecheckBundleFilename: MetaField{
			FlagValueP:      new(string),
			FlagName:        "bundle-filename",
			FlagDesc:        "The filename for the gatecheck bundle, a validatable archive of security artifacts",
			EnvKey:          "PORTAGE_GATECHECK_BUNDLE_FILENAME",
			ActionInputName: "gatecheck_bundle_filename",
			ActionType:      "String",
			DefaultValue:    "gatecheck-bundle.tar.gz",
			stringDecoder:   stringToStringDecoder,
			cobraFunc:       stringVarCobraFunc,
		},
		ImageBuildEnabled: MetaField{
			FlagValueP:      new(bool),
			FlagName:        "enabled",
			FlagDesc:        "Enable/Disable the image build pipeline",
			EnvKey:          "PORTAGE_IMAGE_BUILD_ENABLED",
			ActionInputName: "image_build_enabled",
			ActionType:      "Bool",
			DefaultValue:    "true",
			stringDecoder:   stringToBoolDecoder,
			cobraFunc:       boolVarCobraFunc,
		},
		ImageBuildBuildDir: MetaField{
			FlagValueP:      new(string),
			FlagName:        "build-dir",
			FlagDesc:        "The build directory to use during an image build",
			EnvKey:          "PORTAGE_IMAGE_BUILD_DIR",
			ActionInputName: "build_dir",
			ActionType:      "String",
			DefaultValue:    ".",
			stringDecoder:   stringToStringDecoder,
			cobraFunc:       stringVarCobraFunc,
		},
		ImageBuildDockerfile: MetaField{
			FlagValueP:      new(string),
			FlagName:        "dockerfile",
			FlagDesc:        "The Dockerfile/Containerfile to use during an image build",
			EnvKey:          "PORTAGE_IMAGE_BUILD_DOCKERFILE",
			ActionInputName: "dockerfile",
			ActionType:      "String",
			DefaultValue:    "Dockerfile",
			stringDecoder:   stringToStringDecoder,
			cobraFunc:       stringVarCobraFunc,
		},
		ImageBuildPlatform: MetaField{
			FlagValueP:      new(string),
			FlagName:        "platform",
			FlagDesc:        "The target platform for build",
			EnvKey:          "PORTAGE_IMAGE_BUILD_PLATFORM",
			ActionInputName: "platform",
			ActionType:      "String",
			DefaultValue:    "",
			stringDecoder:   stringToStringDecoder,
			cobraFunc:       stringVarCobraFunc,
		},
		ImageBuildTarget: MetaField{
			FlagValueP:      new(string),
			FlagName:        "target",
			FlagDesc:        "The target build stage to build (e.g., [linux/amd64])",
			EnvKey:          "PORTAGE_IMAGE_BUILD_TARGET",
			ActionInputName: "target",
			ActionType:      "String",
			DefaultValue:    "",
			stringDecoder:   stringToStringDecoder,
			cobraFunc:       stringVarCobraFunc,
		},
		ImageBuildCacheTo: MetaField{
			FlagValueP:      new(string),
			FlagName:        "cache-to",
			FlagDesc:        "Cache export destinations (e.g., \"user/app:cache\", \"type=local,src=path/to/dir\")",
			EnvKey:          "PORTAGE_IMAGE_BUILD_CACHE_TO",
			ActionInputName: "cache_to",
			ActionType:      "String",
			DefaultValue:    "",
			stringDecoder:   stringToStringDecoder,
			cobraFunc:       stringVarCobraFunc,
		},
		ImageBuildCacheFrom: MetaField{
			FlagValueP:      new(string),
			FlagName:        "cache-from",
			FlagDesc:        "External cache sources (e.g., \"user/app:cache\", \"type=local,src=path/to/dir\")",
			EnvKey:          "PORTAGE_IMAGE_BUILD_CACHE_FROM",
			ActionInputName: "cache_from",
			ActionType:      "String",
			DefaultValue:    "",
			stringDecoder:   stringToStringDecoder,
			cobraFunc:       stringVarCobraFunc,
		},
		ImageBuildSquashLayers: MetaField{
			FlagValueP:      new(bool),
			FlagName:        "squash-layers",
			FlagDesc:        "squash image layers - Only Supported with Podman CLI",
			EnvKey:          "PORTAGE_IMAGE_BUILD_SQUASH_LAYERS",
			ActionInputName: "squash_layers",
			ActionType:      "Bool",
			DefaultValue:    "false",
			stringDecoder:   stringToBoolDecoder,
			cobraFunc:       boolVarCobraFunc,
		},
		ImageBuildArgs: MetaField{
			FlagValueP:      new(string),
			FlagName:        "build-args",
			FlagDesc:        "docker build arguments as a json map (ex. '{\"key_1\":\"value\"}')",
			EnvKey:          "PORTAGE_IMAGE_BUILD_ARGS",
			ActionInputName: "build_args",
			ActionType:      "String",
			DefaultValue:    "",
			stringDecoder:   stringToStringDecoder,
			cobraFunc:       stringVarCobraFunc,
		},
		ImageScanEnabled: MetaField{
			FlagValueP:      new(bool),
			FlagName:        "enabled",
			FlagDesc:        "Enable/Disable the image scan pipeline",
			EnvKey:          "PORTAGE_IMAGE_SCAN_ENABLED",
			ActionInputName: "image_scan_enabled",
			ActionType:      "Bool",
			DefaultValue:    "true",
			stringDecoder:   stringToBoolDecoder,
			cobraFunc:       boolVarCobraFunc,
		},
		ImageScanSyftFilename: MetaField{
			FlagValueP:      new(string),
			FlagName:        "syft-filename",
			FlagDesc:        "The filename for the syft SBOM report - must contain 'syft'",
			EnvKey:          "PORTAGE_IMAGE_SCAN_SYFT_FILENAME",
			ActionInputName: "syft_filename",
			ActionType:      "String",
			DefaultValue:    "sbom-report.syft.json",
			stringDecoder:   stringToStringDecoder,
			cobraFunc:       stringVarCobraFunc,
		},
		ImageScanGrypeConfigFilename: MetaField{
			FlagValueP:      new(string),
			FlagName:        "grype-config-filename",
			FlagDesc:        "The config filename for the grype vulnerability report",
			EnvKey:          "PORTAGE_IMAGE_SCAN_GRYPE_CONFIG_FILENAME",
			ActionInputName: "grype_config_filename",
			ActionType:      "String",
			DefaultValue:    "",
			stringDecoder:   stringToStringDecoder,
			cobraFunc:       stringVarCobraFunc,
		},
		ImageScanGrypeFilename: MetaField{
			FlagValueP:      new(string),
			FlagName:        "grype-filename",
			FlagDesc:        "The filename for the grype vulnerability report - must contain 'grype'",
			EnvKey:          "PORTAGE_IMAGE_SCAN_GRYPE_FILENAME",
			ActionInputName: "grype_filename",
			ActionType:      "String",
			DefaultValue:    "image-vulnerability-report.grype.json",
			stringDecoder:   stringToStringDecoder,
			cobraFunc:       stringVarCobraFunc,
		},
		ImageScanClamavFilename: MetaField{
			FlagValueP:      new(string),
			FlagName:        "clamav-filename",
			FlagDesc:        "The filename for the clamscan virus report - must contain 'clamav'",
			EnvKey:          "PORTAGE_IMAGE_SCAN_CLAMAV_FILENAME",
			ActionInputName: "clamav_filename",
			ActionType:      "String",
			DefaultValue:    "virus-report.clamav.txt",
			stringDecoder:   stringToStringDecoder,
			cobraFunc:       stringVarCobraFunc,
		},
		ImageScanFreshclamDisabled: MetaField{
			FlagValueP:      new(bool),
			FlagName:        "no-freshclam",
			FlagDesc:        "enable/disable freshclam database update",
			EnvKey:          "PORTAGE_IMAGE_SCAN_CLAMAV_FRESHCLAM_DISABLED",
			ActionInputName: "clamav_freshclam_disabled",
			ActionType:      "Bool",
			DefaultValue:    "false",
			stringDecoder:   stringToBoolDecoder,
			cobraFunc:       boolVarCobraFunc,
		},
		CodeScanEnabled: MetaField{
			FlagValueP:      new(bool),
			FlagName:        "enabled",
			FlagDesc:        "Enable/Disable the code scan pipeline",
			EnvKey:          "PORTAGE_CODE_SCAN_ENABLED",
			ActionInputName: "code_scan_enabled",
			ActionType:      "Bool",
			DefaultValue:    "true",
			stringDecoder:   stringToBoolDecoder,
			cobraFunc:       boolVarCobraFunc,
		},
		CodeScanSemgrepFilename: MetaField{
			FlagValueP:      new(string),
			FlagName:        "semgrep-filename",
			FlagDesc:        "The filename for the semgrep code scan report - must contain 'gitleaks'",
			EnvKey:          "PORTAGE_CODE_SCAN_SEMGREP_FILENAME",
			ActionInputName: "semgrep_filename",
			ActionType:      "String",
			DefaultValue:    "code-scan-report.semgrep.json",
			stringDecoder:   stringToStringDecoder,
			cobraFunc:       stringVarCobraFunc,
		},
		CodeScanGitleaksFilename: MetaField{
			FlagValueP:      new(string),
			FlagName:        "gitleaks-filename",
			FlagDesc:        "The filename for the gitleaks secret report - must contain 'gitleaks'",
			EnvKey:          "PORTAGE_CODE_SCAN_GITLEAKS_FILENAME",
			ActionInputName: "gitleaks_filename",
			ActionType:      "String",
			DefaultValue:    "secrets-report.gitleaks.json",
			stringDecoder:   stringToStringDecoder,
			cobraFunc:       stringVarCobraFunc,
		},
		CodeScanGitleaksSrcDir: MetaField{
			FlagValueP:      new(string),
			FlagName:        "gitleaks-src-dir",
			FlagDesc:        "The target directory for the gitleaks scan",
			EnvKey:          "PORTAGE_CODE_SCAN_GITLEAKS_SRC_DIR",
			ActionInputName: "gitleaks_src_dir",
			ActionType:      "String",
			DefaultValue:    ".",
			stringDecoder:   stringToStringDecoder,
			cobraFunc:       stringVarCobraFunc,
		},
		CodeScanSemgrepRules: MetaField{
			FlagValueP:      new(string),
			FlagName:        "semgrep-rules",
			FlagDesc:        "rules for semgrep SAST code scan",
			EnvKey:          "PORTAGE_CODE_SCAN_SEMGREP_RULES",
			ActionInputName: "semgrep_rules",
			ActionType:      "String",
			DefaultValue:    "p/auto",
			stringDecoder:   stringToStringDecoder,
			cobraFunc:       stringVarCobraFunc,
		},
		CodeScanSemgrepExperimental: MetaField{
			FlagValueP:      new(bool),
			FlagName:        "semgrep-experimental",
			FlagDesc:        "Enable the use of the semgrep experimental CLI",
			EnvKey:          "PORTAGE_CODE_SCAN_SEMGREP_EXPERIMENTAL",
			ActionInputName: "semgrep_experimental",
			ActionType:      "Bool",
			DefaultValue:    "false",
			stringDecoder:   stringToBoolDecoder,
			cobraFunc:       boolVarCobraFunc,
		},
		CodeScanSemgrepSrcDir: MetaField{
			FlagValueP:      new(string),
			FlagName:        "semgrep-src-dir",
			FlagDesc:        "The target directory for the semgrep scan",
			EnvKey:          "PORTAGE_CODE_SCAN_SEMGREP_SRC_DIR",
			ActionInputName: "semgrep_src_dir",
			ActionType:      "String",
			DefaultValue:    ".",
			stringDecoder:   stringToStringDecoder,
			cobraFunc:       stringVarCobraFunc,
		},
		CodeScanSnykSrcDir: MetaField{
			FlagValueP:      new(string),
			FlagName:        "snyk-src-dir",
			FlagDesc:        "source directory for the scan",
			EnvKey:          "PORTAGE_CODE_SCAN_SNYK_SRC_DIR",
			ActionInputName: "snyk_src_dir",
			ActionType:      "String",
			DefaultValue:    ".",
			stringDecoder:   stringToStringDecoder,
			cobraFunc:       stringVarCobraFunc,
		},
		CodeScanSnykFilename: MetaField{
			FlagValueP:      new(string),
			FlagName:        "snyk-filename",
			FlagDesc:        "The filename for snyk code report",
			EnvKey:          "PORTAGE_CODE_SCAN_SNYK_FILENAME",
			ActionInputName: "snyk_code_filename",
			ActionType:      "String",
			DefaultValue:    "code-scan-report.snyk.sarif.json",
			stringDecoder:   stringToStringDecoder,
			cobraFunc:       stringVarCobraFunc,
		},
		CodeScanCoverageFile: MetaField{
			FlagValueP:      new(string),
			FlagName:        "coverage-file",
			FlagDesc:        "An externally generated code coverage file to validate",
			EnvKey:          "PORTAGE_CODE_SCAN_COVERAGE_FILE",
			ActionInputName: "code_coverage_file",
			ActionType:      "String",
			DefaultValue:    "",
			stringDecoder:   stringToStringDecoder,
			cobraFunc:       stringVarCobraFunc,
		},
		ImagePublishEnabled: MetaField{
			FlagValueP:      new(bool),
			FlagName:        "enabled",
			FlagDesc:        "Enable/Disable the image publish pipeline",
			EnvKey:          "PORTAGE_IMAGE_PUBLISH_ENABLED",
			ActionInputName: "image_publish_enabled",
			ActionType:      "Bool",
			DefaultValue:    "true",
			stringDecoder:   stringToBoolDecoder,
			cobraFunc:       boolVarCobraFunc,
		},
		ImagePublishBundleTag: MetaField{
			FlagValueP:      new(string),
			FlagName:        "bundle-tag",
			FlagDesc:        "The full image tag for the target gatecheck bundle image blob",
			EnvKey:          "PORTAGE_IMAGE_PUBLISH_BUNDLE_TAG",
			ActionInputName: "bundle_publish_tag",
			ActionType:      "String",
			DefaultValue:    "",
			stringDecoder:   stringToStringDecoder,
			cobraFunc:       stringVarCobraFunc,
		},
		ValidationEnabled: MetaField{
			FlagValueP:      new(bool),
			FlagName:        "enabled",
			FlagDesc:        "Enable/Disable the validation pipeline",
			EnvKey:          "PORTAGE_DEPLOY_ENABLED",
			ActionInputName: "validation_enabled",
			ActionType:      "Bool",
			DefaultValue:    "true",
			stringDecoder:   stringToBoolDecoder,
			cobraFunc:       boolVarCobraFunc,
		},
		ValidationGatecheckConfigFilename: MetaField{
			FlagValueP:      new(string),
			FlagName:        "gatecheck-config-filename",
			FlagDesc:        "The filename for the gatecheck config",
			EnvKey:          "PORTAGE_DEPLOY_GATECHECK_CONFIG_FILENAME",
			ActionInputName: "gatecheck_config_filename",
			ActionType:      "String",
			DefaultValue:    "",
			stringDecoder:   stringToStringDecoder,
			cobraFunc:       stringVarCobraFunc,
		},
	}

	return m
}
