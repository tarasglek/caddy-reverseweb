package reversebin

// Note: The legacy CGI-based test has been removed as reverse-bin now operates
// as a process-managing reverse proxy rather than a CGI handler.
// See integration_test.go for the new test suite that validates:
// - Basic reverse proxy functionality
// - Unix socket proxy support
// - Dynamic discovery via detector scripts
// - Lifecycle management (idle timeout, cleanup)
