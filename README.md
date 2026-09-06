# YStreamUtils Plugin Repository

[![Scheduled Plugin Sync](https://github.com/YStreamUtils/YStreamUtils-Plugin-Registry/actions/workflows/plugin-sync.yml/badge.svg)](https://github.com/YStreamUtils/YStreamUtils-Plugin-Registry/actions/workflows/plugin-sync.yml)

[![Plugin Registry Validation](https://github.com/YStreamUtils/YStreamUtils-Plugin-Registry/actions/workflows/plugin-check.yml/badge.svg)](https://github.com/YStreamUtils/YStreamUtils-Plugin-Registry/actions/workflows/plugin-check.yml)

[![Compile Registry Distribution](https://github.com/YStreamUtils/YStreamUtils-Plugin-Registry/actions/workflows/publish-registry.yml/badge.svg)](https://github.com/YStreamUtils/YStreamUtils-Plugin-Registry/actions/workflows/publish-registry.yml)

## CURRENTLY VERY WIP, I HAVE MOST OF IT WORKING I THINK?

## The Stream Companion Plugin Repository You Didn't Ask For

This is where my "official" plugin repository is.

It has a [master registry](https://ystreamutils.github.io/YStreamUtils-Plugin-Registry/registry.json) that the application expects.

Plugins can be submitted by opening a PR. For reference, please check [This PR](https://github.com/YStreamUtils/YStreamUtils-Plugin-Registry/pull/1).

## Generally speaking

New plugins should say something like "plugin_name init at version whatever".

Plugin updates happen automatically on the hour, should be automatically merged.

Major version bumps and new plugins require manual approval.

## Plugin Development

Plugins can be either ts or js, doesn't matter.

Manifests must match the pattern shown in [test=plugin](https://github.com/YStreamUtils/YStreamUtils-Plugin-Registry/blob/master/plugins/YStreamUtils/test-plugin/manifest.json)
or [in the go source for the linter](https://github.com/YStreamUtils/YStreamUtils-Plugin-Registry/blob/master/ci/types/models.go).

The "entry point" is the main file, exports will ONLY be taken from there.

More information in the [test-plugin repo](https://github.com/YStreamUtils/test-plugin).
<!-- <style>h1,h2,h3,h4 { border-bottom: 0; } </style> -->
