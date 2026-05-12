"""
Defines a multiarch_go_binary rule that builds Go binaries for multiple architectures.
"""

load("@rules_go//go:def.bzl", "go_binary")
load("//third_party/bazel:archs.bzl", "LINUX_ARCHES")
load("//third_party/bazel:multiarch_transition.bzl", "platform_transition_cfg")

def _multiarch_binary_impl(ctx):
    binary_files = ctx.attr.binary[0][DefaultInfo].files.to_list()

    executable = None
    for f in binary_files:
        if not f.path.endswith(".a") and not f.path.endswith(".x") and not f.path.endswith("_manifest") and not f.path.endswith("_mapping"):
            executable = f
            break

    if not executable:
        fail("Could not find executable in binary output")

    output = ctx.actions.declare_file(ctx.label.name)

    ctx.actions.run_shell(
        inputs = [executable],
        outputs = [output],
        command = "cp \"$1\" \"$2\" && chmod 755 \"$2\"",
        arguments = [
            executable.path,
            output.path,
        ],
    )

    return [DefaultInfo(
        files = depset([output]),
        executable = output,
    )]

multiarch_binary = rule(
    implementation = _multiarch_binary_impl,
    attrs = {
        "binary": attr.label(mandatory = True, cfg = platform_transition_cfg),
        "arch": attr.string(mandatory = True),
        "_allowlist_function_transition": attr.label(
            default = "@bazel_tools//tools/allowlists/function_transition_allowlist",
        ),
    },
)

def multiarch_go_binary(name, embed, x_defs, arches = LINUX_ARCHES, visibility = None):
    go_binary(
        name = "%s_base" % name,
        embed = embed,
        x_defs = x_defs,
        visibility = ["//visibility:private"],
    )

    for arch in arches:
        multiarch_binary(
            name = "%s_%s" % (name, arch["name"]),
            binary = ":%s_base" % name,
            arch = arch["goarch"],
            visibility = visibility,
        )
