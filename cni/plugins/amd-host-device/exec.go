/*
Copyright (c) Advanced Micro Devices, Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the \"License\");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an \"AS IS\" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/containernetworking/cni/pkg/skel"

	current "github.com/containernetworking/cni/pkg/types/100"
)

var defaultExec = &invoke.DefaultExec{
	RawExec: &invoke.RawExec{Stderr: os.Stderr},
}

func execPlugin(plugin string, command string, confBytes []byte, args *skel.CmdArgs, withResult bool) (*current.Result, error) {
	pluginArgs := &invoke.Args{
		Command:       command,
		ContainerID:   args.ContainerID,
		NetNS:         args.Netns,
		IfName:        args.IfName,
		PluginArgsStr: args.Args,
		Path:          defaultCNIPluginPath,
	}
	paths := filepath.SplitList(defaultCNIPluginPath)
	pluginPath, err := defaultExec.FindInPath(plugin, paths)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), pluginExecTimeout)
	defer cancel()
	if withResult {
		r, err := invoke.ExecPluginWithResult(ctx, pluginPath, confBytes, pluginArgs, defaultExec)
		if err != nil {
			return nil, err
		}
		return current.NewResultFromResult(r)
	}
	// If we don't need a result, we can use ExecPluginWithoutResult
	err = invoke.ExecPluginWithoutResult(ctx, pluginPath, confBytes, pluginArgs, defaultExec)
	return nil, err
}
