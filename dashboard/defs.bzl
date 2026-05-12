def _exec_filegroup_impl(ctx):
    return [DefaultInfo(files = ctx.attr.src[DefaultInfo].files)]

exec_filegroup = rule(
    implementation = _exec_filegroup_impl,
    attrs = {
        "src": attr.label(
            cfg = "exec",
            mandatory = True,
        ),
    },
)
