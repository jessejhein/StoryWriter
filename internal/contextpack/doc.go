// Package contextpack assembles typed, redacted AI context packets and manifests.
//
// It owns pure context selection, lexical relevance, budgeting, and manifest
// serialization. It does not import HTTP, Git, SQLite, providers, or filesystem
// adapters. Story services load canonical material; action services orchestrate
// registry policy and provider execution.
package contextpack
