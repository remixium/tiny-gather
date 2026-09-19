// Package protocol defines the wire format shared by the server, the human
// client, and external agents.
//
// This is the only public package in the module. It is the contract agent
// implementations code against, so changes here are breaking changes for
// codebases this repository cannot see. Everything sent over the wire is
// consumed by models that are billed per token, so field names stay short and
// omitted-when-empty rather than verbose.
package protocol
