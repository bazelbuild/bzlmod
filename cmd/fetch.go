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
	"github.com/bazelbuild/bzlmod/lockfile"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
)

func init() {
	var fetchAll bool
	fetchCmd := &cobra.Command{
		Use:   "fetch <repo> [<repo2> ...]",
		Short: "Fetches the given repo(s)",
		Long: `Fetches the given repo(s) onto local disk. If the fetch is successful, the path
to the directory where the fetched contents reside will be written to stdout.
If only 1 repo was requested to be fetched, the path is simply written out;
otherwise, the output will be multiple lines, each in the format of
"<repoName> <repoPath>" (without quotes).`,
		Run: func(cmd *cobra.Command, args []string) {
			// TODO: figure out wsDir
			if err := runFetch("wsDir", fetchAll, args); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error: %v", err)
			}
		},
	}

	rootCmd.AddCommand(fetchCmd)
	fetchCmd.Flags().BoolVar(&fetchAll, "all", false, `Fetch all known repos.`)
}

func runFetch(wsDir string, fetchAll bool, repos []string) error {
	p, err := ioutil.ReadFile(lockfile.FileName)
	if err != nil {
		return err
	}
	ws, err := lockfile.LoadWorkspace(wsDir, p)
	if err != nil {
		return err
	}
	if fetchAll {
		for name := range ws.Repos {
			if name != ws.RootRepoName {
				err = singleFetch(name, ws, true)
			}
		}
	} else {
		for _, name := range repos {
			err = singleFetch(name, ws, len(repos) > 1)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func singleFetch(name string, ws *lockfile.Workspace, writeName bool) error {
	path, err := ws.Fetch(name)
	if err != nil {
		return err
	}
	if writeName {
		fmt.Print(name)
	}
	fmt.Println(path)
	return nil
}
