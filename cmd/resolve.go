// Copyright 2020 Google LLC
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

package cmd

import (
	"fmt"
	"os"

	"github.com/bazelbuild/bzlmod/resolve"
	"github.com/spf13/cobra"
)

func init() {
	var vendorDir string
	var registries []string

	resolveCmd := &cobra.Command{
		Use:   "resolve",
		Short: "Resolves dependencies and outputs a WORKSPACE file for Bazel",
		Long: `Sets up the current Bazel workspace by reading the MODULE.bazel file,
resolving transitive dependencies, and outputting a WORKSPACE file.`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := resolve.Resolve(".", vendorDir, registries); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error: %v", err)
			}
		},
	}

	rootCmd.AddCommand(resolveCmd)
	resolveCmd.Flags().StringVar(&vendorDir, "vendor_dir", "",
		`Specifies that dependencies should be "vendored" -- that is, ready to be
checked into source control. The value of this flag should be the name of the
directory under the workspace root where vendored dependencies are expected
to be placed.`)
	resolveCmd.Flags().StringSliceVar(&registries, "registries", nil,
		`The list of Bazel registries to pull dependencies from. Earlier registries have
higher priority.`)
}
