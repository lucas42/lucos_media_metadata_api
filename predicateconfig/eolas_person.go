package predicateconfig

// EolasPersonResolver is called by the ResolveNameToURI function for the
// composer and producer predicates. It is expected to look up an existing
// eolas Person entity by name (case-insensitive), creating one if not found,
// and return its URI.
//
// This variable must be set at application startup (e.g. in main()) before
// the HTTP server begins serving requests. Tests should replace it with a
// lightweight stub that returns deterministic URIs without making real HTTP calls.
var EolasPersonResolver func(name string) (string, error)

// EolasPersonNameResolver is called by the ResolveURIToName function for the
// composer and producer predicates. It is expected to return the canonical
// skos:prefLabel for the given eolas Person URI.
//
// This variable must be set at application startup. Tests should replace it
// with a stub.
var EolasPersonNameResolver func(uri string) (string, error)
