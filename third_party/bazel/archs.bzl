"""Architecture definitions and local platform declarations for Bazel."""

LINUX_ARCHES = [
    {
        "name": "x86_64",
        "goarch": "amd64",
        "platform": "linux_amd64",
        "platform_label": "//third_party/bazel:linux_amd64",
        "constraint": "@platforms//cpu:x86_64",
    },
    {
        "name": "aarch64",
        "goarch": "arm64",
        "platform": "linux_arm64",
        "platform_label": "//third_party/bazel:linux_arm64",
        "constraint": "@platforms//cpu:aarch64",
    },
]

LINUX_PLATFORM_LABEL_BY_GOARCH = {
    "amd64": "//third_party/bazel:linux_amd64",
    "arm64": "//third_party/bazel:linux_arm64",
}
