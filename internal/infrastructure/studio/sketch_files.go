package studio

import (
	"errors"
	"fmt"
	"io"
	"os"
)

const sketchLimit = 16 << 20

// Read reads an input image named on the command line.
func (FeatureFiles) Read(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read image: %w", err)
	}
	return b, nil
}

// PutSketch stores the Excalidraw sketch as <feature-id>.sketch.excalidraw beside the feature document.
func (s FeatureFiles) PutSketch(featureID string, data []byte) (string, error) {
	if !fileID.MatchString(featureID) || len(data) > sketchLimit {
		return "", errors.New("invalid sketch")
	}
	name := featureID + ".sketch.excalidraw"
	err := s.withFiles(false, func(root *os.Root) error { return replaceFeature(root, name, data) })
	return name, err
}

// Sketch reads a stored sketch image.
func (s FeatureFiles) Sketch(featureID, name string) ([]byte, error) {
	if !fileID.MatchString(featureID) || name != featureID+".sketch.excalidraw" {
		return nil, errors.New("invalid sketch name")
	}
	var b []byte
	err := s.withFiles(false, func(root *os.Root) error {
		f, err := root.Open(name)
		if err != nil {
			return fmt.Errorf("open sketch: %w", err)
		}
		defer func() { _ = f.Close() }()
		b, err = io.ReadAll(io.LimitReader(f, sketchLimit))
		if err != nil {
			return fmt.Errorf("read sketch: %w", err)
		}
		return nil
	})
	return b, err
}
