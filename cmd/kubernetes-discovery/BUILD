package(default_visibility = ["//visibility:public"])

licenses(["notice"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
)

go_binary(
    name = "kubernetes-discovery",
    srcs = ["main.go"],
    tags = ["automanaged"],
    deps = [ "//cmd/kube-discovery/app:go_default_library"],
)
