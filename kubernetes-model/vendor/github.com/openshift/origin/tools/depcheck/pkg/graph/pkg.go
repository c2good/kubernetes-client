/**
 * Copyright (C) 2015 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *         http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package graph

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/golang/glog"

	"github.com/gonum/graph/concrete"
)

var (
	// Matches standard goimport format for a package.
	//
	// The following formats will successfully match a valid import path:
	//   - host.tld/repo/pkg
	//   - foo.bar/baz
	//
	// The following formats will fail to match an import path:
	//   - company.com
	//   - company/missing/tld
	//   - fmt
	//   - encoding/json
	baseRepoRegex = regexp.MustCompile("[a-zA-Z0-9]+\\.([a-z0-9])+\\/.+")
)

type Package struct {
	Dir         string
	ImportPath  string
	Imports     []string
	TestImports []string
}

type PackageList struct {
	Packages []Package
}

func (p *PackageList) Add(pkg Package) {
	p.Packages = append(p.Packages, pkg)
}

// BuildGraph receives a list of Go packages and constructs a dependency graph from it.
// Any core library dependencies (fmt, strings, etc.) are not added to the graph.
// Any packages whose import path is contained within a list of "excludes" are not added to the graph.
// Returns a directed graph and a map of package import paths to node ids, or an error.
func BuildGraph(packages *PackageList, roots []string, excludes []string) (*MutableDirectedGraph, error) {
	g := NewMutableDirectedGraph(roots)

	// contains the subset of packages from the set of given packages (and their immediate dependencies)
	// that will actually be included in our graph - any packages in the excludes slice, or that do not
	// do not match the baseRepoRegex pattern will be filtered out from this collection.
	filteredPackages := []Package{}

	// add nodes to graph
	for _, pkg := range packages.Packages {
		if isExcludedPath(pkg.ImportPath, excludes) {
			continue
		}
		if !isValidPackagePath(pkg.ImportPath) {
			continue
		}

		n := &Node{
			Id:         g.NewNodeID(),
			UniqueName: pkg.ImportPath,
			LabelName:  labelNameForNode(pkg.ImportPath),
		}
		err := g.AddNode(n)
		if err != nil {
			return nil, err
		}

		filteredPackages = append(filteredPackages, pkg)
	}

	// validate root names exist
	for _, nodeName := range roots {
		if _, exists := g.NodeByName(nodeName); !exists {
			return nil, fmt.Errorf("no corresponding node found for the root name %q", nodeName)
		}
	}

	// add edges
	for _, pkg := range filteredPackages {
		from, exists := g.NodeByName(pkg.ImportPath)
		if !exists {
			return nil, fmt.Errorf("expected node for package %q was not found in graph", pkg.ImportPath)
		}

		for _, dependency := range append(pkg.Imports, pkg.TestImports...) {
			if isExcludedPath(dependency, excludes) {
				continue
			}
			if !isValidPackagePath(dependency) {
				continue
			}

			to, exists := g.NodeByName(dependency)
			if !exists {
				// if a package imports a dependency that we did not visit
				// while traversing the code tree, ignore it, as it is not
				// required for the root repository to build.
				glog.V(1).Infof("Skipping unvisited (missing) dependency %q, which is imported by package %q", dependency, pkg.ImportPath)
				continue
			}

			if g.HasEdgeFromTo(from, to) {
				continue
			}

			g.SetEdge(concrete.Edge{
				F: from,
				T: to,
			}, 0)
		}
	}

	return g, nil
}

func isExcludedPath(path string, excludes []string) bool {
	for _, exclude := range excludes {
		if strings.HasPrefix(path, exclude) {
			return true
		}
	}

	return false
}

// labelNameForNode trims vendored paths of their full /vendor/ path
func labelNameForNode(importPath string) string {
	segs := strings.Split(importPath, "/vendor/")
	if len(segs) > 1 {
		return segs[1]
	}

	return importPath
}

func isValidPackagePath(path string) bool {
	return baseRepoRegex.Match([]byte(path))
}
