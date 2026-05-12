"""
This file contains the default values for the various dependencies that are built by the bazel build system.
"""

SQLC_VERSION = "1.30.0"
SQLC_SHA256_BY_PLATFORM = {
    "linux_amd64": "468aecee071bfe55e97fcbcac52ea0208eeca444f67736f3b8f0f3d6a106132e",
    "linux_arm64": "dd9ab43b022ba3b3402054f99d7ae6e5efea33c949e869c3c66b214415e0c82d",
    "darwin_amd64": "eb065ca44f02a9500f8e51cb63594a6bbd2486af04d18c0f81efadf7eadf5e29",
    "darwin_arm64": "ff18793b97715d08dde364446f43082a06da87b7797b9ec79ef2b31aeb0894e5",
}
