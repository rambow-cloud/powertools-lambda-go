package parameters

import "github.com/rambow-cloud/powertools-lambda-go/parameters/internal/parameterdefaults"

// ClearCaches clears all initialized default providers. Explicit providers have their
// own ClearCache methods. Default AWS configuration and AppConfig sessions are retained.
func ClearCaches() { parameterdefaults.ClearCaches() }
