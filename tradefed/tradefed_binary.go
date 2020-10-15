// Copyright 2018 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package suite_harness

import (
	"strings"

	"github.com/google/blueprint"

	"android/soong/android"
	"android/soong/java"
)

var pctx = android.NewPackageContext("android/soong/suite_harness")

func init() {
	android.RegisterModuleType("tradefed_binary_host", tradefedBinaryFactory)

	pctx.Import("android/soong/android")
}

type TradefedBinaryProperties struct {
	Short_name                    string
	Full_name                     string
	Version                       string
	Prepend_platform_version_name bool
}

// tradefedBinaryFactory creates an empty module for the tradefed_binary module type,
// which is a java_binary with some additional processing in tradefedBinaryLoadHook.
func tradefedBinaryFactory() android.Module {
	props := &TradefedBinaryProperties{}
	module := java.BinaryHostFactory()
	module.AddProperties(props)
	android.AddLoadHook(module, tradefedBinaryLoadHook(props))

	return module
}

const genSuffix = "-gen"

// tradefedBinaryLoadHook adds extra resources and libraries to tradefed_binary modules.
func tradefedBinaryLoadHook(tfb *TradefedBinaryProperties) func(ctx android.LoadHookContext) {
	return func(ctx android.LoadHookContext) {
		genName := ctx.ModuleName() + genSuffix
		version := tfb.Version
		if tfb.Prepend_platform_version_name {
			version = ctx.Config().PlatformVersionName() + tfb.Version
		}

		// Create a submodule that generates the test-suite-info.properties file
		// and copies DynamicConfig.xml if it is present.
		ctx.CreateModule(tradefedBinaryGenFactory,
			&TradefedBinaryGenProperties{
				Name:       &genName,
				Short_name: tfb.Short_name,
				Full_name:  tfb.Full_name,
				Version:    version,
			})

		props := struct {
			Java_resources []string
			Libs           []string
		}{}

		// Add dependencies required by all tradefed_binary modules.
		props.Libs = []string{
			"tradefed",
			"tradefed-test-framework",
			"loganalysis",
			"hosttestlib",
			"compatibility-host-util",
		}

		// Add the files generated by the submodule created above to the resources.
		props.Java_resources = []string{":" + genName}

		ctx.AppendProperties(&props)

	}
}

type TradefedBinaryGenProperties struct {
	Name       *string
	Short_name string
	Full_name  string
	Version    string
}

type tradefedBinaryGen struct {
	android.ModuleBase

	properties TradefedBinaryGenProperties

	gen android.Paths
}

func tradefedBinaryGenFactory() android.Module {
	tfg := &tradefedBinaryGen{}
	tfg.AddProperties(&tfg.properties)
	android.InitAndroidModule(tfg)
	return tfg
}

func (tfg *tradefedBinaryGen) DepsMutator(android.BottomUpMutatorContext) {}

var tradefedBinaryGenRule = pctx.StaticRule("tradefedBinaryGenRule", blueprint.RuleParams{
	Command: `rm -f $out && touch $out && ` +
		`echo "# This file is auto generated by Android.mk. Do not modify." >> $out && ` +
		`echo "build_number = $$(cat ${buildNumberFile})" >> $out && ` +
		`echo "target_arch = ${arch}" >> $out && ` +
		`echo "name = ${name}" >> $out && ` +
		`echo "fullname = ${fullname}" >> $out && ` +
		`echo "version = ${version}" >> $out`,
}, "buildNumberFile", "arch", "name", "fullname", "version")

func (tfg *tradefedBinaryGen) GenerateAndroidBuildActions(ctx android.ModuleContext) {
	buildNumberFile := ctx.Config().BuildNumberFile(ctx)
	outputFile := android.PathForModuleOut(ctx, "test-suite-info.properties")
	ctx.Build(pctx, android.BuildParams{
		Rule:      tradefedBinaryGenRule,
		Output:    outputFile,
		OrderOnly: android.Paths{buildNumberFile},
		Args: map[string]string{
			"buildNumberFile": buildNumberFile.String(),
			"arch":            ctx.Config().DevicePrimaryArchType().String(),
			"name":            tfg.properties.Short_name,
			"fullname":        tfg.properties.Full_name,
			"version":         tfg.properties.Version,
		},
	})

	tfg.gen = append(tfg.gen, outputFile)

	dynamicConfig := android.ExistentPathForSource(ctx, ctx.ModuleDir(), "DynamicConfig.xml")
	if dynamicConfig.Valid() {
		outputFile := android.PathForModuleOut(ctx, strings.TrimSuffix(ctx.ModuleName(), genSuffix)+".dynamic")
		ctx.Build(pctx, android.BuildParams{
			Rule:   android.Cp,
			Input:  dynamicConfig.Path(),
			Output: outputFile,
		})

		tfg.gen = append(tfg.gen, outputFile)
	}
}

func (tfg *tradefedBinaryGen) Srcs() android.Paths {
	return append(android.Paths(nil), tfg.gen...)
}

var _ android.SourceFileProducer = (*tradefedBinaryGen)(nil)
