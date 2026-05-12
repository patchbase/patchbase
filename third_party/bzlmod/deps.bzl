"""
This module implements bzlmod support for external deps.
"""

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("//third_party:versions.bzl", "SQLC_SHA256_BY_PLATFORM", "SQLC_VERSION")

def _norm_os(os_name):
    n = os_name.lower()
    if n == "mac os x":
        return "darwin"
    return n

def _norm_arch(arch, x64 = "x86_64", arm64 = "arm64"):
    a = arch.lower()
    if a in ["amd64", "x86_64"]:
        return x64
    if a in ["arm64", "aarch64"]:
        return arm64
    return a

def _sqlc_ext_impl(ctx):
    os = _norm_os(ctx.os.name)
    arch = _norm_arch(ctx.os.arch.lower(), "amd64", "arm64")

    asset = "%s_%s" % (os, arch)

    if asset not in SQLC_SHA256_BY_PLATFORM:
        fail("Unsupported host platform for sqlc: os=%s arch=%s" % (os, ctx.os.arch))

    url = "https://github.com/sqlc-dev/sqlc/releases/download/v{v}/sqlc_{v}_{asset}.tar.gz".format(
        v = SQLC_VERSION,
        asset = asset,
    )

    sha256 = SQLC_SHA256_BY_PLATFORM.get(asset.lower())
    if not sha256:
        fail("Missing sha256 for sqlc platform key '%s' (os=%s arch=%s)" % (asset.lower(), ctx.os.name, ctx.os.arch))

    http_archive(
        name = "sqlc",
        url = url,
        sha256 = sha256,
        build_file = Label("//third_party/sqlc:sqlc.BUILD"),
    )

sqlc = module_extension(
    implementation = _sqlc_ext_impl,
    os_dependent = True,
    arch_dependent = True,
)
