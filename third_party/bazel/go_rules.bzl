"""Rules for enhancing Go rules."""

load("@rules_go//go:def.bzl", _go_library = "go_library", _go_test = "go_test")

def go_sources(
        name = "go_srcs",
        extra_srcs = [],
        visibility = ["//visibility:public"],
        include = ["*.go"],
        exclude = ["*_test.go"]):
    """Creates a filegroup of Go sources in the current package plus optional extra src labels.

    Args:
      name: target name (default "go_srcs")
      extra_srcs: list of labels/strings to append (e.g. ["//internal/api/v1/runtimes:go_srcs"])
      visibility: standard Bazel visibility
      include: add these glob patterns (default ["*.go"])
      exclude: exclude these glob patterns (default ["*_test.go"])
    """
    native.filegroup(
        name = name,
        srcs = native.glob(include, exclude = exclude) + extra_srcs,
        visibility = visibility,
    )

def go_api_library(
        name,
        importpath,
        srcs = [],
        deps = [],
        embed = [],
        embedsrcs = [],
        visibility = ["//visibility:public"]):
    """Creates a Go library target for API code in the current package.

    Args:
      name: target name (default "go_api_lib")
      importpath: Go import path for this package
      srcs: list of labels/strings for source files (default [":go_srcs"])
      deps: list of labels/strings for dependencies
      embed: list of labels/strings for embedded libraries
      embedsrcs: list of labels/strings for files used by go:embed directives
      visibility: standard Bazel visibility
    """
    go_sources(include = srcs)
    _go_library(
        name = name,
        srcs = srcs,
        importpath = importpath,
        deps = deps,
        embed = embed,
        embedsrcs = embedsrcs,
        visibility = visibility,
    )

_DEFAULT_INTEGRATION_DATA = [
    "//db/fixtures",
]

_DEFAULT_INTEGRATION_TAGS = ["integration", "no-remote-cache"]

def integration_test(name, srcs, data = [], tags = [], deps = [], size = "small", env = {}, **kwargs):
    """Creates a Go integration test with default fixtures

    Args:
      name: test target name.
      srcs: Go test source files.
      data: extra runtime data labels.
      tags: extra Bazel tags.
      deps: Go dependencies.
      size: Bazel test size.
      env: extra environment variables.
      **kwargs: additional args forwarded to go_test.
    """

    test_env = {}
    test_env.update(env)

    _go_test(
        name = name,
        size = size,
        srcs = srcs,
        env = test_env,
        data = _DEFAULT_INTEGRATION_DATA + data,
        tags = _DEFAULT_INTEGRATION_TAGS + tags,
        deps = deps,
        **kwargs
    )
