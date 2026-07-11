// Package modelchat provides provider-neutral chat completion transport for
// OpenAI-compatible and Ollama profiles.
//
// It owns only shared wire mapping, bounded HTTP I/O, and strict response
// parsing. Profile resolution, prompt policy, and consumer-specific parsing
// remain in calling packages.
package modelchat
