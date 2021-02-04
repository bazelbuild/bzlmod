package common

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Label is a parsed representation of a Bazel "Label", which is an identifier of a BUILD target.
type Label struct {
	// Repo is the name of the repo ("repo" in "@repo//package:target"). If the "@repo" part is entirely missing, then
	// HasRepo would be false, and Repo would be empty (distinguishable from when HasRepo is true and Repo is empty,
	// which is possible for labels like "@//package:target").
	Repo string
	// HasRepo denotes whether the the label has a "repo" part at all (see above).
	HasRepo bool
	// Package is the name of the package ("package" in "@repo//package:target"). If the "@repo//package" part is
	// entirely missing, then HasPackage would be false, and Package would be empty (distinguishable from when
	// HasPackage is true and Package is empty, which is possible for labels like "//:target").
	Package string
	// HasPackage denotes whether the the label has a "package" part at all (see above). HasRepo implies HasPackage.
	HasPackage bool
	// Target is the name of the target ("target" in "@repo//package:target"). Target is never empty, since valid target
	// names cannot be empty, and if the ":target" part is entirely missing, Target would be the same as the last
	// segment of the package name (because "//my/package" is a shorthand for "//my/package:package").
	Target string
}

func (l *Label) String() string {
	var s string
	if l.HasRepo {
		s = "@" + l.Repo
	}
	if l.HasPackage {
		s += "//" + l.Package
		if strings.HasSuffix(l.Package, "/"+l.Target) {
			return s
		}
	}
	return s + ":" + l.Target
}

// submatch indices:        0 12 3            4              5
var re = regexp.MustCompile(`^((@([^/\\]*))?//([^:\\]*))?(?::([^:\\]+))?$`)

var ErrParsingLabel = errors.New("error parsing label")

func ParseLabel(raw string) (*Label, error) {
	parsingError := func(reason string) error {
		return fmt.Errorf("%w %q: %v", ErrParsingLabel, raw, reason)
	}

	// These strings would pass our regexp above, but make no sense as labels.
	if raw == "" {
		return nil, parsingError("empty label")
	}
	if raw == "//" {
		return nil, parsingError("malformed label")
	}

	matches := re.FindStringSubmatch(raw)
	if len(matches) != 6 {
		return nil, parsingError("malformed label")
	}
	label := &Label{
		Repo:       matches[3],
		HasRepo:    matches[2] != "",
		Package:    matches[4],
		HasPackage: matches[1] != "",
		Target:     matches[5],
	}

	if label.Package[0] == '/' {
		return nil, parsingError("package names may not start with '/'")
	}
	if strings.HasSuffix(label.Package, "/") {
		return nil, parsingError("package names may not end with '/'")
	}
	if strings.Contains(label.Package, "//") {
		return nil, parsingError("package names may not contain '//'")
	}
	packageSplit := strings.Split(label.Package, "/")
	for _, segment := range packageSplit {
		if strings.TrimLeft(segment, ".") == "" {
			return nil, parsingError("package name component contains only '.' characters")
		}
	}

	if label.Target == "" {
		label.Target = packageSplit[len(packageSplit)-1]
	} else {
		if label.Target[0] == '/' {
			return nil, parsingError("target names may not start with '/'")
		}
		if strings.HasSuffix(label.Target, "/") {
			return nil, parsingError("target names may not end with '/'")
		}
		if strings.Contains(label.Target, "//") {
			return nil, parsingError("target names may not contain '//'")
		}
		for _, segment := range strings.Split(label.Target, "/") {
			if strings.TrimLeft(segment, ".") == "" {
				return nil, parsingError("target name component contains only '.' characters")
			}
		}
	}
	return label, nil
}

type ResolveLabelResult struct {
	Repo     string
	Package  string
	Filename string
}

// LabelResolver knows how to resolve a Label to a file path. Given the current repo, the current package, and a Label,
// LabelResolver returns the repo, package, and file path that the Label points to.
type LabelResolver interface {
	ResolveLabel(curRepo string, curPackage string, label *Label) (*ResolveLabelResult, error)
}
