package application

import "fmt"

// SDKScaffold installs the editor-facing extension SDK declarations
// beside the repository's extensions.
type SDKScaffold interface {
	Write() (paths []string, err error)
}

// InitializeExtensionSDK gives extension authors full editor typing
// with no package manager: the SDK declarations land next to their
// code, generated from the same host types the runtime enforces.
type InitializeExtensionSDK struct {
	scaffold SDKScaffold
}

// NewInitializeExtensionSDK requires the scaffold port.
func NewInitializeExtensionSDK(scaffold SDKScaffold) (InitializeExtensionSDK, error) {
	if scaffold == nil {
		return InitializeExtensionSDK{}, fmt.Errorf("initialize extension sdk: missing scaffold")
	}
	return InitializeExtensionSDK{scaffold: scaffold}, nil
}

// Execute writes the declarations, returning the written paths.
func (uc InitializeExtensionSDK) Execute() ([]string, error) {
	paths, err := uc.scaffold.Write()
	if err != nil {
		return nil, fmt.Errorf("write extension SDK: %w", err)
	}
	return paths, nil
}
