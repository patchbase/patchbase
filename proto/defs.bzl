"""
This module adds go-validate support on top of rules_proto_grpc.
"""

load(
    "@rules_proto_grpc//:defs.bzl",
    "ProtoPluginInfo",
    "proto_compile_attrs",
    "proto_compile_impl",
    "proto_compile_toolchains",
)

go_proto_compile = rule(
    implementation = proto_compile_impl,
    attrs = dict(
        proto_compile_attrs,
        _plugins = attr.label_list(
            providers = [ProtoPluginInfo],
            default = [
                Label("@rules_proto_grpc_go//:proto_plugin"),
                Label("//proto:validate_plugin"),
            ],
            cfg = "exec",
            doc = "List of protoc plugins to apply",
        ),
    ),
    toolchains = proto_compile_toolchains,
)
