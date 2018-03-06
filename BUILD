load("@io_bazel_rules_go//go:def.bzl", "gazelle", "go_binary", "go_library")

gazelle(
    name = "gazelle",
    prefix = "aqwari.net/cmd/gogive",
)

go_library(
    name = "go_default_library",
    srcs = ["gogive.go"],
    importpath = "aqwari.net/cmd/gogive",
    visibility = ["//visibility:private"],
)

go_binary(
    name = "gogive",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
)
