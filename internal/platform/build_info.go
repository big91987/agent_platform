package platform

// Release installation supplies the exact reviewed source revision through Go's
// standard -ldflags -X. Ordinary developer builds keep an explicit non-release value.
const Version = "0.1.0"

var BuildRevision = "development"
