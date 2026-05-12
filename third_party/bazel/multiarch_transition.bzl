"""
Bazel Starlark transition for setting platform based on architecture.
"""

load("//third_party/bazel:archs.bzl", "LINUX_PLATFORM_LABEL_BY_GOARCH")

def _platform_transition_impl(_settings, attr):
    return {"//command_line_option:platforms": [LINUX_PLATFORM_LABEL_BY_GOARCH[attr.arch]]}

platform_transition_cfg = transition(
    implementation = _platform_transition_impl,
    inputs = [],
    outputs = ["//command_line_option:platforms"],
)
